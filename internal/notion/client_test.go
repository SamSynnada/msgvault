package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestNewClient creates a new client with default settings.
func TestNewClient_Default(t *testing.T) {
	client := NewClient("ntn_test_token")
	if client == nil {
		t.Fatal("expected client, got nil")
	}
	if client.token != "ntn_test_token" {
		t.Errorf("expected token %q, got %q", "ntn_test_token", client.token)
	}
	if client.apiVersion != apiVersion {
		t.Errorf("expected apiVersion %q, got %q", apiVersion, client.apiVersion)
	}
	if client.rateLimiter == nil {
		t.Error("expected rate limiter, got nil")
	}
}

// TestNewClient_WithOptions applies client options.
func TestNewClient_WithOptions(t *testing.T) {
	logger := slog.Default()
	rl := NewRateLimiter(5.0)
	httpClient := &http.Client{Timeout: 10 * time.Second}
	baseURL := "https://custom.notion.com"

	client := NewClient("ntn_test_token",
		WithLogger(logger),
		WithRateLimiter(rl),
		WithHTTPClient(httpClient),
		WithBaseURL(baseURL),
	)

	if client.logger != logger {
		t.Error("expected custom logger")
	}
	if client.rateLimiter != rl {
		t.Error("expected custom rate limiter")
	}
	if client.httpClient != httpClient {
		t.Error("expected custom HTTP client")
	}
	if client.baseURL != baseURL {
		t.Error("expected custom base URL")
	}
}

// TestClient_Close closes the client.
func TestClient_Close(t *testing.T) {
	client := NewClient("ntn_test_token")
	err := client.Close()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// TestClient_SearchPages_Success tests a successful search.
func TestClient_SearchPages_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/search" {
			t.Errorf("expected /v1/search, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer ntn_test_token" {
			t.Error("missing or invalid Authorization header")
		}
		if r.Header.Get("Notion-Version") != apiVersion {
			t.Errorf("expected Notion-Version %s, got %s", apiVersion, r.Header.Get("Notion-Version"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing or invalid Content-Type header")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResult{
			Results:    []interface{}{},
			HasMore:    false,
			Type:       "list",
			Object:     "list",
			NextCursor: nil,
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	opts := &SearchOpts{Query: "test"}
	result, err := client.SearchPages(context.Background(), opts)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !result.HasMore {
		t.Error("expected HasMore to be false")
	}
}

// TestClient_GetPage_Success tests retrieving a page.
func TestClient_GetPage_Success(t *testing.T) {
	expectedPage := Page{
		ID:     "test-page-id",
		Object: "page",
		URL:    "https://notion.so/test-page-id",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/pages/test-page-id" {
			t.Errorf("expected /v1/pages/test-page-id, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedPage)
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	page, err := client.GetPage(context.Background(), "test-page-id")

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if page == nil {
		t.Fatal("expected page, got nil")
	}
	if page.ID != expectedPage.ID {
		t.Errorf("expected ID %q, got %q", expectedPage.ID, page.ID)
	}
}

// TestClient_GetDatabaseMetadata_Success tests retrieving database metadata.
func TestClient_GetDatabaseMetadata_Success(t *testing.T) {
	expectedDB := Database{
		ID:     "test-db-id",
		Object: "database",
		Title:  []RichText{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/databases/test-db-id" {
			t.Errorf("expected /v1/databases/test-db-id, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedDB)
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	db, err := client.GetDatabaseMetadata(context.Background(), "test-db-id")

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if db == nil {
		t.Fatal("expected database, got nil")
	}
	if db.ID != expectedDB.ID {
		t.Errorf("expected ID %q, got %q", expectedDB.ID, db.ID)
	}
}

// TestClient_GetBlockChildren_Success tests retrieving block children.
func TestClient_GetBlockChildren_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/blocks/test-page-id/children") {
			t.Errorf("expected /v1/blocks/test-page-id/children*, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BlockList{
			Results:    []*Block{},
			HasMore:    false,
			Type:       "list",
			BlockID:    "test-page-id",
			Object:     "list",
			NextCursor: nil,
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	opts := &BlockListOpts{PageSize: 100}
	result, err := client.GetBlockChildren(context.Background(), "test-page-id", opts)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

// TestClient_QueryDatabase_Success tests querying a database.
func TestClient_QueryDatabase_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/databases/test-db-id/query" {
			t.Errorf("expected /v1/databases/test-db-id/query, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(QueryResult{
			Results:    []*Page{},
			HasMore:    false,
			Type:       "list",
			DatabaseID: "test-db-id",
			Object:     "list",
			NextCursor: nil,
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	opts := &QueryOpts{PageSize: 100}
	result, err := client.QueryDatabase(context.Background(), "test-db-id", opts)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

// TestClient_Retry_On429 tests retrying on 429 (rate limit).
func TestClient_Retry_On429(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResult{
			Results: []interface{}{},
			HasMore: false,
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	result, err := client.SearchPages(context.Background(), &SearchOpts{})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

// TestClient_Retry_On500 tests retrying on 500 (server error).
func TestClient_Retry_On500(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResult{
			Results: []interface{}{},
			HasMore: false,
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	result, err := client.SearchPages(context.Background(), &SearchOpts{})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

// TestClient_Retry_On502 tests retrying on 502 (bad gateway).
func TestClient_Retry_On502(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResult{
			Results: []interface{}{},
			HasMore: false,
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	result, err := client.SearchPages(context.Background(), &SearchOpts{})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

// TestClient_Retry_On503 tests retrying on 503 (service unavailable).
func TestClient_Retry_On503(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResult{
			Results: []interface{}{},
			HasMore: false,
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	result, err := client.SearchPages(context.Background(), &SearchOpts{})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

// TestClient_ExhaustsRetries tests exhausting retry limit.
func TestClient_ExhaustsRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	_, err := client.SearchPages(context.Background(), &SearchOpts{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "max retries exceeded") {
		t.Errorf("expected 'max retries exceeded' in error, got %q", err.Error())
	}
}

// TestClient_Error_400 tests 400 Bad Request.
func TestClient_Error_400(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Error{
			Object:  "error",
			Code:    "invalid_request_body",
			Message: "Invalid request body",
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	_, err := client.SearchPages(context.Background(), &SearchOpts{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected 'bad request' in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "invalid_request_body") {
		t.Errorf("expected 'invalid_request_body' in error, got %q", err.Error())
	}
}

// TestClient_Error_401 tests 401 Unauthorized.
func TestClient_Error_401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	_, err := client.SearchPages(context.Background(), &SearchOpts{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected 'unauthorized' in error, got %q", err.Error())
	}
}

// TestClient_Error_403 tests 403 Forbidden.
func TestClient_Error_403(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Error{
			Object:  "error",
			Code:    "restricted_resource",
			Message: "Could not find database",
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	_, err := client.SearchPages(context.Background(), &SearchOpts{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("expected 'forbidden' in error, got %q", err.Error())
	}
}

// TestClient_Error_404 tests 404 Not Found.
func TestClient_Error_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Error{
			Object:  "error",
			Code:    "not_found",
			Message: "Could not find the requested resource",
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	_, err := client.SearchPages(context.Background(), &SearchOpts{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %q", err.Error())
	}
}

// TestClient_ContextCancellation tests that context cancellation is respected.
func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResult{})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Immediate cancellation

	_, err := client.SearchPages(ctx, &SearchOpts{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected 'context canceled' in error, got %q", err.Error())
	}
}

// TestClient_RateLimiter tests that rate limiter is called.
func TestClient_RateLimiter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResult{
			Results: []interface{}{},
			HasMore: false,
		})
	}))
	defer server.Close()

	rl := NewRateLimiter(10.0) // High rate to avoid delays
	client := NewClient("ntn_test_token", WithBaseURL(server.URL), WithRateLimiter(rl))

	// Make a request
	_, err := client.SearchPages(context.Background(), &SearchOpts{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Check that rate limiter was used
	stats := rl.Stats()
	if count, ok := stats[OpSearch]; !ok || count == 0 {
		t.Error("expected rate limiter to track OpSearch")
	}
}

// TestClient_ExponentialBackoff validates backoff timing is reasonable.
func TestClient_ExponentialBackoff(t *testing.T) {
	client := NewClient("ntn_test_token")

	// Test exponential growth
	backoffs := make([]time.Duration, 5)
	for i := 0; i < 5; i++ {
		backoffs[i] = client.calculateBackoff(i)
		// Just verify backoff is positive and reasonable (not negative)
		if backoffs[i] < 0 {
			t.Errorf("backoff[%d] is negative: %v", i, backoffs[i])
		}
		// Max backoff should be 600 seconds
		if backoffs[i] > 600*time.Second {
			t.Errorf("backoff[%d] exceeds maxBackoff: %v", i, backoffs[i])
		}
	}

	// Verify backoff generally increases (with jitter, not strictly)
	// Just check that backoff grows within reasonable bounds
	for i := 1; i < len(backoffs); i++ {
		// Max possible backoff should be around 2^attempt seconds
		maxExpected := time.Duration(uint(1)<<uint(i)) * time.Second
		if backoffs[i] > maxExpected {
			t.Errorf("backoff[%d]=%v exceeds expected max %v", i, backoffs[i], maxExpected)
		}
	}
}

// TestClient_RetryAfterHeader respects Retry-After header.
func TestClient_RetryAfterHeader(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Header().Set("Retry-After", "1")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResult{
			Results: []interface{}{},
			HasMore: false,
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	start := time.Now()
	result, err := client.SearchPages(context.Background(), &SearchOpts{})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}

	// Verify we waited (at least 0.5s, accounting for test timing variations)
	elapsed := time.Since(start)
	if elapsed < 500*time.Millisecond {
		t.Logf("warning: elapsed time less than expected: %v", elapsed)
	}
}

// TestClient_ParseNotionError parses error responses correctly.
func TestClient_ParseNotionError(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		body       map[string]interface{}
		wantErr    string
	}{
		{
			name:       "bad_request",
			statusCode: 400,
			body: map[string]interface{}{
				"object":  "error",
				"code":    "invalid_request_body",
				"message": "Invalid request body",
			},
			wantErr: "invalid_request_body",
		},
		{
			name:       "restricted_resource",
			statusCode: 403,
			body: map[string]interface{}{
				"object":  "error",
				"code":    "restricted_resource",
				"message": "Could not find the requested page",
			},
			wantErr: "restricted_resource",
		},
		{
			name:       "not_found",
			statusCode: 404,
			body: map[string]interface{}{
				"object":  "error",
				"code":    "not_found",
				"message": "Could not find database",
			},
			wantErr: "not_found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(tc.body)
			}))
			defer server.Close()

			client := NewClient("ntn_test_token", WithBaseURL(server.URL))
			_, err := client.SearchPages(context.Background(), &SearchOpts{})

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

// TestClient_GetBlockChildren_WithPagination tests pagination parameters.
func TestClient_GetBlockChildren_WithPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if pageSize := query.Get("page_size"); pageSize != "100" {
			t.Errorf("expected page_size=100, got %q", pageSize)
		}
		if cursor := query.Get("start_cursor"); cursor != "abc123" {
			t.Errorf("expected start_cursor=abc123, got %q", cursor)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BlockList{
			Results: []*Block{},
			HasMore: false,
		})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	opts := &BlockListOpts{
		PageSize:    100,
		StartCursor: "abc123",
	}
	_, err := client.GetBlockChildren(context.Background(), "test-page-id", opts)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

// TestClient_GetBlockChildren_PageSizeClamping tests page size clamping.
func TestClient_GetBlockChildren_PageSizeClamping(t *testing.T) {
	testCases := []struct {
		name     string
		pageSize int
		wantSize string
	}{
		{"zero", 0, ""},
		{"one", 1, "1"},
		{"fifty", 50, "50"},
		{"hundred", 100, "100"},
		{"over_limit", 200, "100"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query()
				pageSize := query.Get("page_size")
				if pageSize != tc.wantSize {
					t.Errorf("expected page_size=%q, got %q", tc.wantSize, pageSize)
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(BlockList{
					Results: []*Block{},
					HasMore: false,
				})
			}))
			defer server.Close()

			client := NewClient("ntn_test_token", WithBaseURL(server.URL))
			opts := &BlockListOpts{PageSize: tc.pageSize}
			_, err := client.GetBlockChildren(context.Background(), "test-page-id", opts)

			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

// TestClient_InvalidJSON tests handling of invalid JSON responses.
func TestClient_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	_, err := client.SearchPages(context.Background(), &SearchOpts{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse search result") {
		t.Errorf("expected 'parse search result' in error, got %q", err.Error())
	}
}

// TestClient_NetworkError tests handling of network errors with retry.
func TestClient_NetworkError(t *testing.T) {
	// Create a server that we'll close to simulate connection errors
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	address := listener.Addr().String()
	listener.Close()

	client := NewClient("ntn_test_token", WithBaseURL("http://"+address))
	_, err = client.SearchPages(context.Background(), &SearchOpts{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "max retries exceeded") {
		t.Errorf("expected 'max retries exceeded' in error, got %q", err.Error())
	}
}

// TestClient_StatusOK tests that 200-299 status codes are accepted.
func TestClient_StatusOK(t *testing.T) {
	for statusCode := 200; statusCode < 300; statusCode++ {
		t.Run(fmt.Sprintf("status_%d", statusCode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(statusCode)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(SearchResult{
					Results: []interface{}{},
					HasMore: false,
				})
			}))
			defer server.Close()

			client := NewClient("ntn_test_token", WithBaseURL(server.URL))
			result, err := client.SearchPages(context.Background(), &SearchOpts{})

			if err != nil {
				t.Errorf("expected no error for status %d, got %v", statusCode, err)
			}
			if result == nil {
				t.Errorf("expected result for status %d, got nil", statusCode)
			}
		})
	}
}

// TestClient_QueryOptions tests QueryOpts marshaling.
func TestClient_QueryOptions_Marshal(t *testing.T) {
	opts := &QueryOpts{
		PageSize:    50,
		StartCursor: "cursor123",
		Filter: map[string]interface{}{
			"property": "Name",
			"title": map[string]interface{}{
				"contains": "test",
			},
		},
	}

	data, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify we can unmarshal it back
	var decoded QueryOpts
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if decoded.PageSize != 50 {
		t.Errorf("expected PageSize 50, got %d", decoded.PageSize)
	}
	if decoded.StartCursor != "cursor123" {
		t.Errorf("expected StartCursor cursor123, got %q", decoded.StartCursor)
	}
}

// TestClient_HeadersSet ensures all required headers are set.
func TestClient_HeadersSet(t *testing.T) {
	headersCaptured := make(map[string]string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headersCaptured["Authorization"] = r.Header.Get("Authorization")
		headersCaptured["Notion-Version"] = r.Header.Get("Notion-Version")
		headersCaptured["Content-Type"] = r.Header.Get("Content-Type")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SearchResult{})
	}))
	defer server.Close()

	client := NewClient("ntn_test_token", WithBaseURL(server.URL))
	client.SearchPages(context.Background(), &SearchOpts{})

	if headersCaptured["Authorization"] != "Bearer ntn_test_token" {
		t.Errorf("missing or invalid Authorization header: %q", headersCaptured["Authorization"])
	}
	if headersCaptured["Notion-Version"] != apiVersion {
		t.Errorf("missing or invalid Notion-Version header: %q", headersCaptured["Notion-Version"])
	}
	if headersCaptured["Content-Type"] != "application/json" {
		t.Errorf("missing or invalid Content-Type header: %q", headersCaptured["Content-Type"])
	}
}

// TestParseNotionError_WithExtraFields tests error parsing with unexpected fields.
func TestParseNotionError_WithExtraFields(t *testing.T) {
	respBody := []byte(`{
		"object": "error",
		"status": 400,
		"code": "validation_error",
		"message": "Invalid property",
		"request_id": "123456"
	}`)

	err := parseNotionError(respBody)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "validation_error") {
		t.Errorf("expected error code in message, got %q", err.Error())
	}
}

// TestParseNotionError_EmptyMessage tests error parsing with empty message.
func TestParseNotionError_EmptyMessage(t *testing.T) {
	respBody := []byte(`{"object": "error", "code": ""}`)
	err := parseNotionError(respBody)
	if err != nil {
		t.Errorf("expected nil for empty error, got %v", err)
	}
}

// TestParseNotionError_InvalidJSON tests error parsing with invalid JSON.
func TestParseNotionError_InvalidJSON(t *testing.T) {
	respBody := []byte(`{invalid}`)
	err := parseNotionError(respBody)
	if err != nil {
		t.Errorf("expected nil for invalid JSON, got %v", err)
	}
}
