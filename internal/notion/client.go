// Package notion provides a client for the Notion API v2025-09-03.
package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const (
	baseURL        = "https://api.notion.com/v1"
	apiVersion     = "2025-09-03"
	maxRetries     = 6  // Covers ~10 minutes of outages (1s + 2s + 4s + 8s + 16s + 32s + 64s + 128s + 256s ≈ 511s)
	maxBackoff     = 600 // Max backoff in seconds (10 minutes)
	defaultTimeout = 30 * time.Second
)

// NotionAPI defines the Notion API client interface.
type NotionAPI interface {
	SearchPages(ctx context.Context, opts *SearchOpts) (*SearchResult, error)
	GetPage(ctx context.Context, pageID string) (*Page, error)
	GetDatabaseMetadata(ctx context.Context, dbID string) (*Database, error)
	GetBlockChildren(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error)
	QueryDatabase(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error)
	Close() error
}

// ConcreteClient implements the Notion API interface.
type ConcreteClient struct {
	httpClient  *http.Client
	baseURL     string
	token       string
	apiVersion  string
	rateLimiter *RateLimiter
	logger      *slog.Logger
}

// ClientOption configures a ConcreteClient.
type ClientOption func(*ConcreteClient)

// WithLogger sets the logger for the client.
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *ConcreteClient) {
		c.logger = logger
	}
}

// WithRateLimiter sets a custom rate limiter.
func WithRateLimiter(rl *RateLimiter) ClientOption {
	return func(c *ConcreteClient) {
		c.rateLimiter = rl
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *ConcreteClient) {
		c.httpClient = httpClient
	}
}

// WithBaseURL sets a custom base URL (useful for testing).
func WithBaseURL(url string) ClientOption {
	return func(c *ConcreteClient) {
		c.baseURL = url
	}
}

// NewClient creates a new Notion API client.
func NewClient(token string, opts ...ClientOption) *ConcreteClient {
	c := &ConcreteClient{
		baseURL:    baseURL,
		token:      token,
		apiVersion: apiVersion,
		logger:     slog.Default(),
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	// Default rate limiter if not set
	if c.rateLimiter == nil {
		c.rateLimiter = NewRateLimiter(DefaultTokensPerSec)
	}

	return c
}

// Close releases resources held by the client.
func (c *ConcreteClient) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}

// request makes an HTTP request with rate limiting and retry logic.
// bodyBytes can be nil for requests without a body.
func (c *ConcreteClient) request(ctx context.Context, op Operation, method, path string, bodyBytes []byte) ([]byte, error) {
	// Acquire rate limit token
	if err := c.rateLimiter.Acquire(ctx, op); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}

	reqURL := c.baseURL + path
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := c.calculateBackoff(attempt)
			c.logger.Debug("retrying request", "attempt", attempt, "backoff", backoff, "path", path)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// Create a new reader for each attempt to ensure body can be re-read on retry
		var body io.Reader
		if bodyBytes != nil {
			body = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		// Set common headers
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Notion-Version", c.apiVersion)
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			continue // Retry on network errors
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response: %w", err)
			continue
		}

		// Check for success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}

		// Handle specific error codes
		switch resp.StatusCode {
		case 400: // Bad Request
			notionErr := parseNotionError(respBody)
			return nil, fmt.Errorf("bad request (400): %w", notionErr)

		case 401: // Unauthorized
			return nil, fmt.Errorf("unauthorized (401): invalid or expired token")

		case 403: // Forbidden
			notionErr := parseNotionError(respBody)
			return nil, fmt.Errorf("forbidden (403): %w", notionErr)

		case 404: // Not Found
			notionErr := parseNotionError(respBody)
			return nil, fmt.Errorf("not found (404): %w", notionErr)

		case 429: // Too Many Requests (rate limit)
			c.logger.Debug("rate limited, backing off", "path", path, "attempt", attempt)
			lastErr = fmt.Errorf("rate limited (429)")
			// Check for Retry-After header
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					// Wait the specified time, but cap at maxBackoff
					waitTime := time.Duration(seconds) * time.Second
					if waitTime > time.Duration(maxBackoff)*time.Second {
						waitTime = time.Duration(maxBackoff) * time.Second
					}
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(waitTime):
					}
					continue
				}
			}
			continue // Retry with calculated backoff

		case 500, 502, 503: // Server errors
			lastErr = fmt.Errorf("server error (%d)", resp.StatusCode)
			continue

		default: // Other client errors - don't retry
			notionErr := parseNotionError(respBody)
			if notionErr != nil {
				return nil, fmt.Errorf("request failed (%d): %w", resp.StatusCode, notionErr)
			}
			return nil, fmt.Errorf("request failed (%d): %s", resp.StatusCode, string(respBody))
		}
	}

	return nil, fmt.Errorf("max retries exceeded (%d attempts): %w", maxRetries+1, lastErr)
}

// calculateBackoff returns the backoff duration for a retry attempt.
// Uses exponential backoff with full jitter.
func (c *ConcreteClient) calculateBackoff(attempt int) time.Duration {
	// Exponential: 1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 600, 600...
	base := float64(uint(1) << uint(attempt))
	if base > maxBackoff {
		base = maxBackoff
	}

	// Full jitter: random value between 0 and base
	jittered := rand.Float64() * base
	return time.Duration(jittered * float64(time.Second))
}

// parseNotionError parses a Notion API error from JSON response body.
func parseNotionError(respBody []byte) error {
	var notionErr Error
	if err := json.Unmarshal(respBody, &notionErr); err != nil {
		return nil // Not a Notion error JSON, let caller handle
	}

	if notionErr.Code != "" {
		return fmt.Errorf("notion api error: [%s] %s", notionErr.Code, notionErr.Message)
	}
	if notionErr.Message != "" {
		return fmt.Errorf("notion api error: %s", notionErr.Message)
	}
	return nil
}

// SearchPages searches for pages and databases in the workspace.
func (c *ConcreteClient) SearchPages(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
	body, err := json.Marshal(opts)
	if err != nil {
		return nil, fmt.Errorf("marshal search options: %w", err)
	}

	respBody, err := c.request(ctx, OpSearch, "POST", "/v1/search", body)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse search result: %w", err)
	}

	return &result, nil
}

// GetPage retrieves a single page by ID.
func (c *ConcreteClient) GetPage(ctx context.Context, pageID string) (*Page, error) {
	path := fmt.Sprintf("/v1/pages/%s", pageID)
	respBody, err := c.request(ctx, OpGetPage, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var page Page
	if err := json.Unmarshal(respBody, &page); err != nil {
		return nil, fmt.Errorf("parse page: %w", err)
	}

	return &page, nil
}

// GetDatabaseMetadata retrieves a database's metadata.
func (c *ConcreteClient) GetDatabaseMetadata(ctx context.Context, dbID string) (*Database, error) {
	path := fmt.Sprintf("/v1/databases/%s", dbID)
	respBody, err := c.request(ctx, OpGetPage, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var db Database
	if err := json.Unmarshal(respBody, &db); err != nil {
		return nil, fmt.Errorf("parse database: %w", err)
	}

	return &db, nil
}

// GetBlockChildren retrieves the children blocks of a page or block.
func (c *ConcreteClient) GetBlockChildren(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
	path := fmt.Sprintf("/v1/blocks/%s/children", pageID)

	// Build query parameters
	if opts != nil && (opts.PageSize > 0 || opts.StartCursor != "") {
		path += "?"
		if opts.PageSize > 0 {
			// Clamp page size between 1 and 100
			pageSize := opts.PageSize
			if pageSize < 1 {
				pageSize = 1
			}
			if pageSize > 100 {
				pageSize = 100
			}
			path += fmt.Sprintf("page_size=%d", pageSize)
		}
		if opts.StartCursor != "" {
			if opts.PageSize > 0 {
				path += "&"
			}
			path += fmt.Sprintf("start_cursor=%s", opts.StartCursor)
		}
	}

	respBody, err := c.request(ctx, OpGetBlocks, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var blockList BlockList
	if err := json.Unmarshal(respBody, &blockList); err != nil {
		return nil, fmt.Errorf("parse block list: %w", err)
	}

	return &blockList, nil
}

// QueryDatabase queries a database with filters and sorting.
func (c *ConcreteClient) QueryDatabase(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error) {
	body, err := json.Marshal(opts)
	if err != nil {
		return nil, fmt.Errorf("marshal query options: %w", err)
	}

	path := fmt.Sprintf("/v1/databases/%s/query", dbID)
	respBody, err := c.request(ctx, OpQueryDatabase, "POST", path, body)
	if err != nil {
		return nil, err
	}

	var result QueryResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse query result: %w", err)
	}

	return &result, nil
}

// SearchOpts represents options for searching pages.
type SearchOpts struct {
	Query    string         `json:"query,omitempty"`
	Filter   *SearchFilter  `json:"filter,omitempty"`
	Sort     *SearchSort    `json:"sort,omitempty"`
	PageSize int            `json:"page_size,omitempty"`
	StartCursor string      `json:"start_cursor,omitempty"`
}

// SearchFilter represents a filter for search operations.
type SearchFilter struct {
	Property string `json:"property"`
	Value    string `json:"value"`
}

// SearchSort represents sorting for search results.
type SearchSort struct {
	Direction string `json:"direction"` // "ascending" or "descending"
	Timestamp string `json:"timestamp"` // "last_edited_time" or "created_time"
}

// SearchResult represents the response from a search operation.
type SearchResult struct {
	Results    []interface{} `json:"results"` // Can be Page or Database
	NextCursor *string       `json:"next_cursor"`
	HasMore    bool          `json:"has_more"`
	Type       string        `json:"type"`
	Object     string        `json:"object"`
}

// BlockListOpts represents options for listing blocks.
type BlockListOpts struct {
	PageSize    int
	StartCursor string
}

// BlockList represents a list of blocks response.
type BlockList struct {
	Results    []*Block `json:"results"`
	NextCursor *string  `json:"next_cursor"`
	HasMore    bool     `json:"has_more"`
	Type       string   `json:"type"`
	BlockID    string   `json:"block_id"`
	Object     string   `json:"object"`
}

// QueryOpts represents options for querying a database.
type QueryOpts struct {
	Filter      interface{}  `json:"filter,omitempty"`
	Sorts       interface{}  `json:"sorts,omitempty"`
	PageSize    int          `json:"page_size,omitempty"`
	StartCursor string       `json:"start_cursor,omitempty"`
}

// QueryResult represents the response from a database query.
type QueryResult struct {
	Results    []*Page `json:"results"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	Type       string  `json:"type"`
	DatabaseID string  `json:"database_id"`
	Object     string  `json:"object"`
}
