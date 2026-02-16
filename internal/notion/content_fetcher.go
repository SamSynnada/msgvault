// Package notion provides a content fetcher for retrieving page and database content.
package notion

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Fetcher retrieves page and database content from Notion API.
type Fetcher interface {
	// FetchPageContent retrieves all blocks for a page with pagination support.
	// Returns PageContent with blocks and pagination token for incremental fetching.
	FetchPageContent(ctx context.Context, pageID string) (*PageContent, error)

	// FetchDatabaseRows retrieves rows from a database with optional filtering.
	// Supports timestamp filtering for incremental sync.
	FetchDatabaseRows(ctx context.Context, dbID string, opts *QueryOpts) ([]*DatabaseRow, error)
}

// PageContent represents all blocks for a page.
type PageContent struct {
	// PageID is the ID of the page.
	PageID string

	// Blocks are all blocks in the page (flat list).
	Blocks []*Block

	// HasMore indicates if there are more blocks to fetch.
	HasMore bool

	// NextCursor is the pagination cursor for fetching more blocks.
	NextCursor string
}

// DatabaseRow represents a row in a Notion database.
type DatabaseRow struct {
	// ID is the unique identifier for the row (page).
	ID string

	// Title is the primary title of the row (extracted from title property).
	Title string

	// Properties contains all database properties for the row.
	Properties map[string]interface{}

	// URL is the Notion URL for the row.
	URL string
}

// fetcher implements the Fetcher interface.
type fetcher struct {
	client NotionAPI
	logger *slog.Logger
}

// NewFetcher creates a new content fetcher.
// Panics if client or logger is nil.
func NewFetcher(client NotionAPI, logger *slog.Logger) Fetcher {
	if client == nil {
		panic("fetcher: client is required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &fetcher{
		client: client,
		logger: logger,
	}
}

// FetchPageContent retrieves all blocks for a page with full pagination.
// Handles the 100-block limit per request by making multiple requests.
func (f *fetcher) FetchPageContent(ctx context.Context, pageID string) (*PageContent, error) {
	// Context check
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if pageID == "" {
		return nil, fmt.Errorf("fetcher: page ID cannot be empty")
	}

	f.logger.Debug("fetching page content", "page_id", pageID)

	var allBlocks []*Block
	var hasMore bool
	var nextCursor string

	// Paginate through all blocks (100 per request)
	opts := &BlockListOpts{
		PageSize: 100,
	}

	for {
		// Check context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Set cursor for pagination
		if nextCursor != "" {
			opts.StartCursor = nextCursor
		}

		// Fetch blocks
		blockList, err := f.client.GetBlockChildren(ctx, pageID, opts)
		if err != nil {
			return nil, fmt.Errorf("fetch blocks for page %s: %w", pageID, err)
		}

		if blockList == nil {
			return nil, fmt.Errorf("unexpected nil block list for page %s", pageID)
		}

		allBlocks = append(allBlocks, blockList.Results...)
		f.logger.Debug("fetched blocks", "page_id", pageID, "count", len(blockList.Results))

		// Check if we need to paginate further
		if !blockList.HasMore {
			hasMore = false
			nextCursor = ""
			break
		}

		// If no cursor provided despite HasMore=true, can't continue
		if blockList.NextCursor == nil {
			f.logger.Warn("has_more=true but no next_cursor", "page_id", pageID)
			hasMore = true
			nextCursor = ""
			break
		}

		nextCursor = *blockList.NextCursor
		hasMore = true
	}

	pc := &PageContent{
		PageID:     pageID,
		Blocks:     allBlocks,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}

	f.logger.Debug("fetched page content complete", "page_id", pageID, "total_blocks", len(allBlocks), "has_more", hasMore)
	return pc, nil
}

// FetchDatabaseRows queries a database and returns rows with extracted titles.
// Supports timestamp filtering for incremental sync.
func (f *fetcher) FetchDatabaseRows(ctx context.Context, dbID string, opts *QueryOpts) ([]*DatabaseRow, error) {
	// Context check
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if dbID == "" {
		return nil, fmt.Errorf("fetcher: database ID cannot be empty")
	}

	// Use default page size if not specified
	if opts == nil {
		opts = &QueryOpts{
			PageSize: 100,
		}
	}
	if opts.PageSize == 0 {
		opts.PageSize = 100
	}

	f.logger.Debug("querying database rows", "db_id", dbID, "page_size", opts.PageSize)

	// Query database
	queryResult, err := f.client.QueryDatabase(ctx, dbID, opts)
	if err != nil {
		return nil, fmt.Errorf("query database %s: %w", dbID, err)
	}

	if queryResult == nil {
		return nil, fmt.Errorf("unexpected nil query result for database %s", dbID)
	}

	// Convert pages to database rows
	rows := make([]*DatabaseRow, 0, len(queryResult.Results))
	for i, page := range queryResult.Results {
		if page == nil {
			f.logger.Warn("nil page in query result", "db_id", dbID, "index", i)
			continue
		}

		row := &DatabaseRow{
			ID:         page.ID,
			Title:      extractDatabaseTitle(page),
			Properties: page.Properties,
			URL:        page.URL,
		}

		rows = append(rows, row)
	}

	f.logger.Debug("queried database rows", "db_id", dbID, "count", len(rows))
	return rows, nil
}

// extractDatabaseTitle extracts the title from a database row (page).
// Searches for a title property in the page's properties map.
func extractDatabaseTitle(page *Page) string {
	if page == nil || page.Properties == nil {
		return ""
	}

	// Look for common title property names
	titleNames := []string{"Name", "Title", "name", "title"}
	for _, name := range titleNames {
		if prop, exists := page.Properties[name]; exists {
			// Property is typically a complex structure with title array
			if propMap, ok := prop.(map[string]interface{}); ok {
				if titleArray, ok := propMap["title"].([]interface{}); ok && len(titleArray) > 0 {
					if titleObj, ok := titleArray[0].(map[string]interface{}); ok {
						if text, ok := titleObj["plain_text"].(string); ok && text != "" {
							return text
						}
					}
				}
			}
		}
	}

	// Fallback to page URL if no title found
	return ""
}

// QueryDatabaseFilter represents a filter for database queries with timestamp support.
type QueryDatabaseFilter struct {
	// PropertyName is the database property to filter on.
	PropertyName string

	// FilterType is the type of filter: "text", "date", "checkbox", etc.
	FilterType string

	// Value is the filter value.
	Value interface{}
}

// QueryDatabaseSort represents a sort for database queries.
type QueryDatabaseSort struct {
	// PropertyName is the database property to sort on.
	PropertyName string

	// Direction is "ascending" or "descending".
	Direction string
}
