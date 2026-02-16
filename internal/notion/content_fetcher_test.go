// Package notion provides a content fetcher for retrieving page and database content.
package notion

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"
)

// MockClient implements NotionAPI for testing.
type MockClient struct {
	SearchPagesFunc      func(ctx context.Context, opts *SearchOpts) (*SearchResult, error)
	GetPageFunc          func(ctx context.Context, pageID string) (*Page, error)
	GetDatabaseMetadataFunc func(ctx context.Context, dbID string) (*Database, error)
	GetBlockChildrenFunc func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error)
	QueryDatabaseFunc    func(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error)
	CloseFunc            func() error
}

func (m *MockClient) SearchPages(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
	if m.SearchPagesFunc != nil {
		return m.SearchPagesFunc(ctx, opts)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockClient) GetPage(ctx context.Context, pageID string) (*Page, error) {
	if m.GetPageFunc != nil {
		return m.GetPageFunc(ctx, pageID)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockClient) GetDatabaseMetadata(ctx context.Context, dbID string) (*Database, error) {
	if m.GetDatabaseMetadataFunc != nil {
		return m.GetDatabaseMetadataFunc(ctx, dbID)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockClient) GetBlockChildren(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
	if m.GetBlockChildrenFunc != nil {
		return m.GetBlockChildrenFunc(ctx, pageID, opts)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockClient) QueryDatabase(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error) {
	if m.QueryDatabaseFunc != nil {
		return m.QueryDatabaseFunc(ctx, dbID, opts)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *MockClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

// TestNewFetcher tests that NewFetcher creates a fetcher correctly.
func TestNewFetcher(t *testing.T) {
	logger := slog.Default()
	mockClient := &MockClient{}

	f := NewFetcher(mockClient, logger)
	if f == nil {
		t.Error("expected non-nil fetcher")
	}
}

// TestNewFetcher_NilClient tests that NewFetcher panics with nil client.
func TestNewFetcher_NilClient(t *testing.T) {
	logger := slog.Default()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil client")
		}
	}()

	NewFetcher(nil, logger)
}

// TestNewFetcher_DefaultLogger tests that NewFetcher uses default logger when nil.
func TestNewFetcher_DefaultLogger(t *testing.T) {
	mockClient := &MockClient{}

	f := NewFetcher(mockClient, nil)
	if f == nil {
		t.Error("expected non-nil fetcher")
	}
}

// TestFetchPageContent_SinglePage tests fetching a page with one request.
func TestFetchPageContent_SinglePage(t *testing.T) {
	mockClient := &MockClient{
		GetBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			return &BlockList{
				Results: []*Block{
					{
						ID:   "block-1",
						Type: "paragraph",
					},
					{
						ID:   "block-2",
						Type: "heading_1",
					},
				},
				HasMore:    false,
				NextCursor: nil,
			}, nil
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	content, err := f.FetchPageContent(context.Background(), "page-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content == nil {
		t.Fatal("expected non-nil page content")
	}
	if content.PageID != "page-123" {
		t.Errorf("expected page_id 'page-123', got %s", content.PageID)
	}
	if len(content.Blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(content.Blocks))
	}
	if content.HasMore {
		t.Error("expected has_more to be false")
	}
	if content.NextCursor != "" {
		t.Errorf("expected empty next_cursor, got %s", content.NextCursor)
	}
}

// TestFetchPageContent_Pagination tests fetching with pagination across multiple requests.
func TestFetchPageContent_Pagination(t *testing.T) {
	callCount := 0
	mockClient := &MockClient{
		GetBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			callCount++

			// First call: return 2 blocks with more available
			if callCount == 1 {
				cursor := "cursor-1"
				return &BlockList{
					Results: []*Block{
						{ID: "block-1", Type: "paragraph"},
						{ID: "block-2", Type: "heading_1"},
					},
					HasMore:    true,
					NextCursor: &cursor,
				}, nil
			}

			// Second call: return 2 more blocks, no more available
			if callCount == 2 {
				if opts.StartCursor != "cursor-1" {
					t.Errorf("expected start_cursor 'cursor-1', got %s", opts.StartCursor)
				}
				return &BlockList{
					Results: []*Block{
						{ID: "block-3", Type: "bulleted_list_item"},
						{ID: "block-4", Type: "code"},
					},
					HasMore:    false,
					NextCursor: nil,
				}, nil
			}

			return nil, fmt.Errorf("unexpected call")
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	content, err := f.FetchPageContent(context.Background(), "page-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content.Blocks) != 4 {
		t.Errorf("expected 4 blocks total, got %d", len(content.Blocks))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
	if content.HasMore {
		t.Error("expected has_more to be false")
	}
}

// TestFetchPageContent_LargePage tests fetching a large page with many pagination requests.
func TestFetchPageContent_LargePage(t *testing.T) {
	callCount := 0
	mockClient := &MockClient{
		GetBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			callCount++

			// Simulate 10k blocks across 100 requests (100 blocks per request)
			if callCount <= 100 {
				var blocks []*Block
				for i := 0; i < 100; i++ {
					blockID := fmt.Sprintf("block-%d", (callCount-1)*100+i)
					blocks = append(blocks, &Block{ID: blockID, Type: "paragraph"})
				}

				bl := &BlockList{
					Results: blocks,
					HasMore: callCount < 100,
				}

				if callCount < 100 {
					cursor := fmt.Sprintf("cursor-%d", callCount)
					bl.NextCursor = &cursor
				}

				return bl, nil
			}

			return nil, fmt.Errorf("unexpected call")
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	content, err := f.FetchPageContent(context.Background(), "page-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content.Blocks) != 10000 {
		t.Errorf("expected 10000 blocks, got %d", len(content.Blocks))
	}
	if callCount != 100 {
		t.Errorf("expected 100 API calls, got %d", callCount)
	}
}

// TestFetchPageContent_EmptyPage tests fetching a page with no blocks.
func TestFetchPageContent_EmptyPage(t *testing.T) {
	mockClient := &MockClient{
		GetBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			return &BlockList{
				Results:    []*Block{},
				HasMore:    false,
				NextCursor: nil,
			}, nil
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	content, err := f.FetchPageContent(context.Background(), "page-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content.Blocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(content.Blocks))
	}
}

// TestFetchPageContent_EmptyPageID tests error handling for empty page ID.
func TestFetchPageContent_EmptyPageID(t *testing.T) {
	mockClient := &MockClient{}
	f := NewFetcher(mockClient, slog.Default())

	_, err := f.FetchPageContent(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty page ID")
	}
}

// TestFetchPageContent_ClientError tests error propagation from client.
func TestFetchPageContent_ClientError(t *testing.T) {
	mockClient := &MockClient{
		GetBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			return nil, fmt.Errorf("API error")
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	_, err := f.FetchPageContent(context.Background(), "page-123")

	if err == nil {
		t.Fatal("expected error from client")
	}
}

// TestFetchPageContent_ContextCancellation tests context cancellation handling.
func TestFetchPageContent_ContextCancellation(t *testing.T) {
	mockClient := &MockClient{}
	f := NewFetcher(mockClient, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.FetchPageContent(ctx, "page-123")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

// TestFetchDatabaseRows_SinglePage tests fetching database rows with one request.
func TestFetchDatabaseRows_SinglePage(t *testing.T) {
	mockClient := &MockClient{
		QueryDatabaseFunc: func(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error) {
			return &QueryResult{
				Results: []*Page{
					{
						ID:  "row-1",
						URL: "https://notion.so/row-1",
						Properties: map[string]interface{}{
							"Name": map[string]interface{}{
								"title": []interface{}{
									map[string]interface{}{
										"plain_text": "Task 1",
									},
								},
							},
						},
					},
					{
						ID:  "row-2",
						URL: "https://notion.so/row-2",
						Properties: map[string]interface{}{
							"Title": map[string]interface{}{
								"title": []interface{}{
									map[string]interface{}{
										"plain_text": "Task 2",
									},
								},
							},
						},
					},
				},
				HasMore:    false,
				NextCursor: nil,
			}, nil
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	rows, err := f.FetchDatabaseRows(context.Background(), "db-123", &QueryOpts{PageSize: 100})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Title != "Task 1" {
		t.Errorf("expected title 'Task 1', got %s", rows[0].Title)
	}
	if rows[1].Title != "Task 2" {
		t.Errorf("expected title 'Task 2', got %s", rows[1].Title)
	}
}

// TestFetchDatabaseRows_NoTitle tests handling of rows without title property.
func TestFetchDatabaseRows_NoTitle(t *testing.T) {
	mockClient := &MockClient{
		QueryDatabaseFunc: func(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error) {
			return &QueryResult{
				Results: []*Page{
					{
						ID:         "row-1",
						URL:        "https://notion.so/row-1",
						Properties: map[string]interface{}{},
					},
				},
				HasMore:    false,
				NextCursor: nil,
			}, nil
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	rows, err := f.FetchDatabaseRows(context.Background(), "db-123", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Title != "" {
		t.Errorf("expected empty title, got %s", rows[0].Title)
	}
}

// TestFetchDatabaseRows_EmptyDatabase tests fetching from empty database.
func TestFetchDatabaseRows_EmptyDatabase(t *testing.T) {
	mockClient := &MockClient{
		QueryDatabaseFunc: func(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error) {
			return &QueryResult{
				Results:    []*Page{},
				HasMore:    false,
				NextCursor: nil,
			}, nil
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	rows, err := f.FetchDatabaseRows(context.Background(), "db-123", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

// TestFetchDatabaseRows_EmptyDatabaseID tests error handling for empty database ID.
func TestFetchDatabaseRows_EmptyDatabaseID(t *testing.T) {
	mockClient := &MockClient{}
	f := NewFetcher(mockClient, slog.Default())

	_, err := f.FetchDatabaseRows(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty database ID")
	}
}

// TestFetchDatabaseRows_ClientError tests error propagation from client.
func TestFetchDatabaseRows_ClientError(t *testing.T) {
	mockClient := &MockClient{
		QueryDatabaseFunc: func(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error) {
			return nil, fmt.Errorf("API error")
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	_, err := f.FetchDatabaseRows(context.Background(), "db-123", nil)

	if err == nil {
		t.Fatal("expected error from client")
	}
}

// TestFetchDatabaseRows_ContextCancellation tests context cancellation handling.
func TestFetchDatabaseRows_ContextCancellation(t *testing.T) {
	mockClient := &MockClient{}
	f := NewFetcher(mockClient, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.FetchDatabaseRows(ctx, "db-123", nil)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

// TestFetchDatabaseRows_DefaultPageSize tests that default page size is used.
func TestFetchDatabaseRows_DefaultPageSize(t *testing.T) {
	mockClient := &MockClient{
		QueryDatabaseFunc: func(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error) {
			if opts.PageSize != 100 {
				t.Errorf("expected default page size 100, got %d", opts.PageSize)
			}
			return &QueryResult{
				Results:    []*Page{},
				HasMore:    false,
				NextCursor: nil,
			}, nil
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	_, err := f.FetchDatabaseRows(context.Background(), "db-123", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestFetchDatabaseRows_PreservesOptions tests that query options are preserved.
func TestFetchDatabaseRows_PreservesOptions(t *testing.T) {
	opts := &QueryOpts{
		PageSize: 50,
		Filter: map[string]interface{}{
			"property": "Status",
			"select": map[string]interface{}{
				"equals": "Done",
			},
		},
	}

	mockClient := &MockClient{
		QueryDatabaseFunc: func(ctx context.Context, dbID string, recvOpts *QueryOpts) (*QueryResult, error) {
			if recvOpts.PageSize != 50 {
				t.Errorf("expected page size 50, got %d", recvOpts.PageSize)
			}
			if recvOpts.Filter == nil {
				t.Error("expected filter to be preserved")
			}
			return &QueryResult{
				Results:    []*Page{},
				HasMore:    false,
				NextCursor: nil,
			}, nil
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	_, err := f.FetchDatabaseRows(context.Background(), "db-123", opts)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestExtractDatabaseTitle_SingleProperty tests title extraction from single property.
func TestExtractDatabaseTitle_SingleProperty(t *testing.T) {
	page := &Page{
		Properties: map[string]interface{}{
			"Name": map[string]interface{}{
				"title": []interface{}{
					map[string]interface{}{
						"plain_text": "My Task",
					},
				},
			},
		},
	}

	title := extractDatabaseTitle(page)
	if title != "My Task" {
		t.Errorf("expected 'My Task', got '%s'", title)
	}
}

// TestExtractDatabaseTitle_AlternativeProperty tests title extraction with Title instead of Name.
func TestExtractDatabaseTitle_AlternativeProperty(t *testing.T) {
	page := &Page{
		Properties: map[string]interface{}{
			"Title": map[string]interface{}{
				"title": []interface{}{
					map[string]interface{}{
						"plain_text": "Alternative Title",
					},
				},
			},
		},
	}

	title := extractDatabaseTitle(page)
	if title != "Alternative Title" {
		t.Errorf("expected 'Alternative Title', got '%s'", title)
	}
}

// TestExtractDatabaseTitle_Empty tests title extraction from empty page.
func TestExtractDatabaseTitle_Empty(t *testing.T) {
	page := &Page{
		Properties: map[string]interface{}{},
	}

	title := extractDatabaseTitle(page)
	if title != "" {
		t.Errorf("expected empty title, got '%s'", title)
	}
}

// TestExtractDatabaseTitle_Nil tests title extraction from nil page.
func TestExtractDatabaseTitle_Nil(t *testing.T) {
	var page *Page
	title := extractDatabaseTitle(page)
	if title != "" {
		t.Errorf("expected empty title, got '%s'", title)
	}
}

// TestFetchPageContent_PaginationContinuation tests pagination with intermediate cursor.
func TestFetchPageContent_PaginationContinuation(t *testing.T) {
	callCount := 0
	mockClient := &MockClient{
		GetBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			callCount++

			if callCount == 1 {
				cursor := "cursor-1"
				return &BlockList{
					Results: []*Block{
						{ID: "block-1", Type: "paragraph"},
					},
					HasMore:    true,
					NextCursor: &cursor,
				}, nil
			}

			if callCount == 2 {
				cursor := "cursor-2"
				return &BlockList{
					Results: []*Block{
						{ID: "block-2", Type: "paragraph"},
					},
					HasMore:    true,
					NextCursor: &cursor,
				}, nil
			}

			if callCount == 3 {
				return &BlockList{
					Results: []*Block{
						{ID: "block-3", Type: "paragraph"},
					},
					HasMore:    false,
					NextCursor: nil,
				}, nil
			}

			return nil, fmt.Errorf("unexpected call")
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	content, err := f.FetchPageContent(context.Background(), "page-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content.Blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(content.Blocks))
	}
	if callCount != 3 {
		t.Errorf("expected 3 API calls, got %d", callCount)
	}
}

// TestFetchPageContent_HasMoreWithoutCursor tests handling of has_more without cursor.
func TestFetchPageContent_HasMoreWithoutCursor(t *testing.T) {
	mockClient := &MockClient{
		GetBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			return &BlockList{
				Results: []*Block{
					{ID: "block-1", Type: "paragraph"},
				},
				HasMore:    true,
				NextCursor: nil,
			}, nil
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	content, err := f.FetchPageContent(context.Background(), "page-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content.Blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(content.Blocks))
	}
	// HasMore should be set even without cursor
	if !content.HasMore {
		t.Error("expected has_more to be true")
	}
}

// TestFetchPageContent_ConcurrentContextCancellation tests concurrent context cancellation.
func TestFetchPageContent_ConcurrentContextCancellation(t *testing.T) {
	mockClient := &MockClient{
		GetBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			// Simulate a delay to allow context cancellation
			select {
			case <-time.After(100 * time.Millisecond):
				return &BlockList{
					Results:    []*Block{},
					HasMore:    false,
					NextCursor: nil,
				}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}

	f := NewFetcher(mockClient, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := f.FetchPageContent(ctx, "page-123")
	if err == nil {
		t.Fatal("expected context deadline exceeded error")
	}
}

// TestFetchDatabaseRows_NilResultProperty tests handling of nil properties in results.
func TestFetchDatabaseRows_NilResultProperty(t *testing.T) {
	mockClient := &MockClient{
		QueryDatabaseFunc: func(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error) {
			return &QueryResult{
				Results: []*Page{
					{
						ID:  "row-1",
						URL: "https://notion.so/row-1",
					},
					nil, // nil page in results
					{
						ID:  "row-2",
						URL: "https://notion.so/row-2",
						Properties: map[string]interface{}{
							"Name": map[string]interface{}{
								"title": []interface{}{
									map[string]interface{}{
										"plain_text": "Task 2",
									},
								},
							},
						},
					},
				},
				HasMore:    false,
				NextCursor: nil,
			}, nil
		},
	}

	f := NewFetcher(mockClient, slog.Default())
	rows, err := f.FetchDatabaseRows(context.Background(), "db-123", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should skip nil page and only return 2 valid rows
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
}

// TestPageContent_Struct tests PageContent structure fields.
func TestPageContent_Struct(t *testing.T) {
	pc := &PageContent{
		PageID:     "page-123",
		Blocks:     []*Block{{ID: "b1"}},
		HasMore:    true,
		NextCursor: "cursor-123",
	}

	if pc.PageID != "page-123" {
		t.Errorf("expected page_id 'page-123', got %s", pc.PageID)
	}
	if len(pc.Blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(pc.Blocks))
	}
	if !pc.HasMore {
		t.Error("expected has_more to be true")
	}
	if pc.NextCursor != "cursor-123" {
		t.Errorf("expected cursor 'cursor-123', got %s", pc.NextCursor)
	}
}

// TestDatabaseRow_Struct tests DatabaseRow structure fields.
func TestDatabaseRow_Struct(t *testing.T) {
	props := map[string]interface{}{"key": "value"}
	dr := &DatabaseRow{
		ID:         "row-123",
		Title:      "Test Row",
		Properties: props,
		URL:        "https://notion.so/row",
	}

	if dr.ID != "row-123" {
		t.Errorf("expected id 'row-123', got %s", dr.ID)
	}
	if dr.Title != "Test Row" {
		t.Errorf("expected title 'Test Row', got %s", dr.Title)
	}
	if len(dr.Properties) != 1 {
		t.Errorf("expected 1 property, got %d", len(dr.Properties))
	}
	if dr.URL != "https://notion.so/row" {
		t.Errorf("expected URL 'https://notion.so/row', got %s", dr.URL)
	}
}
