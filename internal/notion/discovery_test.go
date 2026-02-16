package notion

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"
)

// MockNotionAPI is a mock implementation of NotionAPI for testing.
type MockNotionAPI struct {
	searchPagesFunc         func(context.Context, *SearchOpts) (*SearchResult, error)
	getPageFunc             func(context.Context, string) (*Page, error)
	getDatabaseMetadataFunc func(context.Context, string) (*Database, error)
	getBlockChildrenFunc    func(context.Context, string, *BlockListOpts) (*BlockList, error)
	queryDatabaseFunc       func(context.Context, string, *QueryOpts) (*QueryResult, error)
}

func (m *MockNotionAPI) SearchPages(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
	if m.searchPagesFunc != nil {
		return m.searchPagesFunc(ctx, opts)
	}
	return &SearchResult{Results: []interface{}{}}, nil
}

func (m *MockNotionAPI) GetPage(ctx context.Context, pageID string) (*Page, error) {
	if m.getPageFunc != nil {
		return m.getPageFunc(ctx, pageID)
	}
	return nil, nil
}

func (m *MockNotionAPI) GetDatabaseMetadata(ctx context.Context, dbID string) (*Database, error) {
	if m.getDatabaseMetadataFunc != nil {
		return m.getDatabaseMetadataFunc(ctx, dbID)
	}
	return nil, nil
}

func (m *MockNotionAPI) GetBlockChildren(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
	if m.getBlockChildrenFunc != nil {
		return m.getBlockChildrenFunc(ctx, pageID, opts)
	}
	return &BlockList{Results: []*Block{}}, nil
}

func (m *MockNotionAPI) QueryDatabase(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error) {
	if m.queryDatabaseFunc != nil {
		return m.queryDatabaseFunc(ctx, dbID, opts)
	}
	return nil, nil
}

func (m *MockNotionAPI) Close() error {
	return nil
}

func TestDiscoverer_DiscoverPages_Empty(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			return &SearchResult{
				Results: []interface{}{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	count := 0
	for range pageChan {
		count++
	}

	if count != 0 {
		t.Errorf("Expected 0 pages, got %d", count)
	}
}

func TestDiscoverer_DiscoverPages_SinglePage(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			if opts.StartCursor != "" {
				return &SearchResult{HasMore: false}, nil
			}

			page := &Page{
				ID:             "page1",
				Object:         "page",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/page1",
				Properties: map[string]interface{}{
					"title": map[string]interface{}{
						"title": []interface{}{
							map[string]interface{}{
								"plain_text": "Test Page",
							},
						},
					},
				},
			}

			return &SearchResult{
				Results: []interface{}{page},
				HasMore: false,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			return &BlockList{
				Results: []*Block{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	var pages []*PageInfo
	for page := range pageChan {
		pages = append(pages, page)
	}

	if len(pages) != 1 {
		t.Errorf("Expected 1 page, got %d", len(pages))
	}

	if pages[0].ID != "page1" {
		t.Errorf("Expected page ID 'page1', got %q", pages[0].ID)
	}

	if pages[0].Title != "Test Page" {
		t.Errorf("Expected title 'Test Page', got %q", pages[0].Title)
	}

	if pages[0].IsDatabase {
		t.Error("Expected page, got database")
	}
}

func TestDiscoverer_DiscoverPages_Pagination(t *testing.T) {
	calls := 0
	nextCursor := "next_page"

	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			calls++

			if calls == 1 {
				page1 := &Page{
					ID:             "page1",
					Object:         "page",
					CreatedTime:    time.Now(),
					LastEditedTime: time.Now(),
					URL:            "https://notion.so/page1",
					Properties:     map[string]interface{}{},
				}

				return &SearchResult{
					Results:    []interface{}{page1},
					NextCursor: &nextCursor,
					HasMore:    true,
				}, nil
			}

			// Second call
			page2 := &Page{
				ID:             "page2",
				Object:         "page",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/page2",
				Properties:     map[string]interface{}{},
			}

			return &SearchResult{
				Results: []interface{}{page2},
				HasMore: false,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			return &BlockList{
				Results: []*Block{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	var pages []*PageInfo
	for page := range pageChan {
		pages = append(pages, page)
	}

	if len(pages) != 2 {
		t.Errorf("Expected 2 pages from pagination, got %d", len(pages))
	}

	// 2 calls for the paginated root search, plus 1 from discoverTopLevelPagesViaSearch
	if calls != 3 {
		t.Errorf("Expected 3 API calls, got %d", calls)
	}
}

func TestDiscoverer_DiscoverPages_ChildPages(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			page := &Page{
				ID:             "parent1",
				Object:         "page",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/parent1",
				Properties:     map[string]interface{}{},
			}

			return &SearchResult{
				Results: []interface{}{page},
				HasMore: false,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			if pageID == "parent1" {
				childBlock := &Block{
					ID:             "child1",
					Type:           "child_page",
					CreatedTime:    time.Now(),
					LastEditedTime: time.Now(),
					ChildPage: &ChildPageBlock{
						Title: "Child Page",
					},
				}

				return &BlockList{
					Results: []*Block{childBlock},
					HasMore: false,
				}, nil
			}

			// For child page
			return &BlockList{
				Results: []*Block{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	var pages []*PageInfo
	for page := range pageChan {
		pages = append(pages, page)
	}

	// Only the parent is sent to the channel; child pages are nested in ChildPages
	if len(pages) != 1 {
		t.Errorf("Expected 1 page on channel (parent only), got %d", len(pages))
	}

	parent := pages[0]
	if parent.ID != "parent1" {
		t.Fatalf("Expected parent page ID 'parent1', got %q", parent.ID)
	}

	if len(parent.ChildPages) != 1 {
		t.Errorf("Expected 1 child page, got %d", len(parent.ChildPages))
	}

	if parent.ChildPages[0].ID != "child1" {
		t.Errorf("Expected child page ID 'child1', got %q", parent.ChildPages[0].ID)
	}

	if parent.ChildPages[0].Title != "Child Page" {
		t.Errorf("Expected child title 'Child Page', got %q", parent.ChildPages[0].Title)
	}
}

func TestDiscoverer_DiscoverPages_ChildDatabases(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			page := &Page{
				ID:             "page1",
				Object:         "page",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/page1",
				Properties:     map[string]interface{}{},
			}

			return &SearchResult{
				Results: []interface{}{page},
				HasMore: false,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			if pageID == "page1" {
				dbBlock := &Block{
					ID:   "db1",
					Type: "child_database",
					ChildDatabase: &ChildDatabaseBlock{
						Title: "Test Database",
					},
				}

				return &BlockList{
					Results: []*Block{dbBlock},
					HasMore: false,
				}, nil
			}

			return &BlockList{
				Results: []*Block{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	var pages []*PageInfo
	for page := range pageChan {
		pages = append(pages, page)
	}

	if len(pages) != 1 {
		t.Errorf("Expected 1 page, got %d", len(pages))
	}

	page := pages[0]
	if len(page.ChildDatabases) != 1 {
		t.Errorf("Expected 1 child database, got %d", len(page.ChildDatabases))
	}

	if page.ChildDatabases[0].ID != "db1" {
		t.Errorf("Expected database ID 'db1', got %q", page.ChildDatabases[0].ID)
	}

	if page.ChildDatabases[0].Title != "Test Database" {
		t.Errorf("Expected database title 'Test Database', got %q", page.ChildDatabases[0].Title)
	}
}

func TestDiscoverer_DiscoverPages_Database(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			db := &Database{
				ID:             "db1",
				Object:         "database",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/db1",
				Title: []RichText{
					{
						PlainText: "Test Database",
					},
				},
			}

			return &SearchResult{
				Results: []interface{}{db},
				HasMore: false,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			return &BlockList{
				Results: []*Block{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	var pages []*PageInfo
	for page := range pageChan {
		pages = append(pages, page)
	}

	if len(pages) != 1 {
		t.Errorf("Expected 1 database, got %d", len(pages))
	}

	page := pages[0]
	if !page.IsDatabase {
		t.Error("Expected database, got page")
	}

	if page.ID != "db1" {
		t.Errorf("Expected database ID 'db1', got %q", page.ID)
	}

	if page.Title != "Test Database" {
		t.Errorf("Expected title 'Test Database', got %q", page.Title)
	}
}

func TestDiscoverer_DiscoverPages_MaxDepth(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			page := &Page{
				ID:             "page1",
				Object:         "page",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/page1",
				Properties:     map[string]interface{}{},
			}

			return &SearchResult{
				Results: []interface{}{page},
				HasMore: false,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			if pageID == "page1" {
				// Create a child page
				childBlock := &Block{
					ID:             "page2",
					Type:           "child_page",
					CreatedTime:    time.Now(),
					LastEditedTime: time.Now(),
					ChildPage: &ChildPageBlock{
						Title: "Page 2",
					},
				}

				return &BlockList{
					Results: []*Block{childBlock},
					HasMore: false,
				}, nil
			}

			if pageID == "page2" {
				// Create another child page (should be blocked by max depth)
				grandchildBlock := &Block{
					ID:             "page3",
					Type:           "child_page",
					CreatedTime:    time.Now(),
					LastEditedTime: time.Now(),
					ChildPage: &ChildPageBlock{
						Title: "Page 3",
					},
				}

				return &BlockList{
					Results: []*Block{grandchildBlock},
					HasMore: false,
				}, nil
			}

			return &BlockList{
				Results: []*Block{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Set max depth to 2 (discover root pages and their direct children, but not grandchildren).
	// MaxDepth=2 means discoverChildBlocks at depth 0 passes (0+1 < 2), but at depth 1
	// it stops (1+1 >= 2), so page3 (grandchild) is not discovered.
	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{MaxDepth: 2})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	var pages []*PageInfo
	for page := range pageChan {
		pages = append(pages, page)
	}

	// Only page1 is sent to the channel; child pages are nested in ChildPages
	if len(pages) != 1 {
		t.Errorf("Expected 1 page on channel with MaxDepth=2, got %d", len(pages))
	}

	// Check that page1's direct child (page2) was discovered, but grandchild (page3) was not
	if len(pages[0].ChildPages) != 1 {
		t.Errorf("Expected 1 child page, got %d", len(pages[0].ChildPages))
	}

	if pages[0].ChildPages[0].ID != "page2" {
		t.Errorf("Expected child page ID 'page2', got %q", pages[0].ChildPages[0].ID)
	}

	// page2 should have no children (grandchild page3 blocked by depth limit)
	if len(pages[0].ChildPages[0].ChildPages) != 0 {
		t.Errorf("Expected 0 grandchild pages (blocked by depth), got %d", len(pages[0].ChildPages[0].ChildPages))
	}
}

func TestDiscoverer_DiscoverPages_Limit(t *testing.T) {
	calls := 0

	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			calls++

			// Return pages as long as limit not reached
			page := &Page{
				ID:             "page" + string(rune('0'+calls)),
				Object:         "page",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/page",
				Properties:     map[string]interface{}{},
			}

			nextCursor := "next"
			return &SearchResult{
				Results:    []interface{}{page},
				NextCursor: &nextCursor,
				HasMore:    true,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			return &BlockList{
				Results: []*Block{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Set limit to 2
	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{Limit: 2})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	count := 0
	for range pageChan {
		count++
	}

	if count != 2 {
		t.Errorf("Expected 2 pages with Limit=2, got %d", count)
	}
}

func TestDiscoverer_DiscoverPages_ContextCancellation(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			// Block to simulate slow operation
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	// Channel should close quickly even though it was cancelled
	count := 0
	for range pageChan {
		count++
	}

	if count != 0 {
		t.Errorf("Expected 0 pages when context cancelled, got %d", count)
	}
}

func TestDiscoverer_DiscoverPages_APIError(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			return nil, errors.New("api error")
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	// Channel should close since search failed
	count := 0
	for range pageChan {
		count++
	}

	if count != 0 {
		t.Errorf("Expected 0 pages after API error, got %d", count)
	}
}

func TestDiscoverer_DiscoverPages_BlockChildrenError(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			page := &Page{
				ID:             "page1",
				Object:         "page",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/page1",
				Properties:     map[string]interface{}{},
			}

			return &SearchResult{
				Results: []interface{}{page},
				HasMore: false,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			// Simulate permission denied error for this page
			return nil, errors.New("permission denied")
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	var pages []*PageInfo
	for page := range pageChan {
		pages = append(pages, page)
	}

	// Should still get the page, just without child blocks
	if len(pages) != 1 {
		t.Errorf("Expected 1 page despite child block error, got %d", len(pages))
	}

	if len(pages[0].ChildPages) != 0 {
		t.Errorf("Expected 0 child pages after error, got %d", len(pages[0].ChildPages))
	}
}

func TestDiscoverer_DiscoverPages_BlockPagination(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			page := &Page{
				ID:             "page1",
				Object:         "page",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/page1",
				Properties:     map[string]interface{}{},
			}

			return &SearchResult{
				Results: []interface{}{page},
				HasMore: false,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			if pageID == "page1" {
				// First call returns first batch with next_cursor
				if opts.StartCursor == "" {
					nextCursor := "next_block"
					childBlock := &Block{
						ID:             "page2",
						Type:           "child_page",
						CreatedTime:    time.Now(),
						LastEditedTime: time.Now(),
						ChildPage: &ChildPageBlock{
							Title: "Page 2",
						},
					}

					return &BlockList{
						Results:    []*Block{childBlock},
						NextCursor: &nextCursor,
						HasMore:    true,
					}, nil
				}

				// Second call returns next batch
				childBlock := &Block{
					ID:             "page3",
					Type:           "child_page",
					CreatedTime:    time.Now(),
					LastEditedTime: time.Now(),
					ChildPage: &ChildPageBlock{
						Title: "Page 3",
					},
				}

				return &BlockList{
					Results: []*Block{childBlock},
					HasMore: false,
				}, nil
			}

			return &BlockList{
				Results: []*Block{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	var pages []*PageInfo
	for page := range pageChan {
		pages = append(pages, page)
	}

	// Only the parent is sent to the channel; child pages are nested in ChildPages
	if len(pages) != 1 {
		t.Errorf("Expected 1 page on channel (parent only), got %d", len(pages))
	}

	parent := pages[0]
	if parent.ID != "page1" {
		t.Fatalf("Expected parent page ID 'page1', got %q", parent.ID)
	}

	// Verify both child pages were discovered across paginated block responses
	if len(parent.ChildPages) != 2 {
		t.Errorf("Expected 2 child pages, got %d", len(parent.ChildPages))
	}
}

func TestDiscoverer_DiscoverPages_SkipDatabases(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			db := &Database{
				ID:             "db1",
				Object:         "database",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/db1",
				Title: []RichText{
					{
						PlainText: "Test Database",
					},
				},
			}

			page := &Page{
				ID:             "page1",
				Object:         "page",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/page1",
				Properties:     map[string]interface{}{},
			}

			return &SearchResult{
				Results: []interface{}{db, page},
				HasMore: false,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			return &BlockList{
				Results: []*Block{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Don't skip databases
	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{SkipDatabases: false})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	var pages []*PageInfo
	for page := range pageChan {
		pages = append(pages, page)
	}

	if len(pages) != 2 {
		t.Errorf("Expected 2 items (1 page + 1 database), got %d", len(pages))
	}
}

func TestDiscoverer_NilLogger(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			return &SearchResult{HasMore: false}, nil
		},
	}

	// Should not panic with nil logger
	discoverer := NewDiscoverer(mock, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	// Drain channel
	for range pageChan {
	}
}

func TestDiscoverer_NilOpts(t *testing.T) {
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			page := &Page{
				ID:             "page1",
				Object:         "page",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/page1",
				Properties:     map[string]interface{}{},
			}

			return &SearchResult{
				Results: []interface{}{page},
				HasMore: false,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			return &BlockList{
				Results: []*Block{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Nil opts should be handled gracefully
	pageChan, err := discoverer.DiscoverPages(ctx, nil)
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	count := 0
	for range pageChan {
		count++
	}

	if count != 1 {
		t.Errorf("Expected 1 page, got %d", count)
	}
}

func TestDiscoverer_ExtractPageTitle_FromProperties(t *testing.T) {
	page := &Page{
		ID:  "page1",
		URL: "https://notion.so/page1",
		Properties: map[string]interface{}{
			"title": map[string]interface{}{
				"title": []interface{}{
					map[string]interface{}{
						"plain_text": "My Page Title",
					},
				},
			},
		},
	}

	mock := &MockNotionAPI{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := &discoverer{client: mock, logger: logger}

	title := discoverer.extractPageTitle(page)
	if title != "My Page Title" {
		t.Errorf("Expected title 'My Page Title', got %q", title)
	}
}

func TestDiscoverer_ExtractPageTitle_Fallback(t *testing.T) {
	page := &Page{
		ID:         "page1",
		URL:        "https://notion.so/page1",
		Properties: nil,
	}

	mock := &MockNotionAPI{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := &discoverer{client: mock, logger: logger}

	title := discoverer.extractPageTitle(page)
	if title != "https://notion.so/page1" {
		t.Errorf("Expected fallback URL, got %q", title)
	}
}

func TestDiscoverer_ExtractDatabaseTitle(t *testing.T) {
	db := &Database{
		ID:  "db1",
		URL: "https://notion.so/db1",
		Title: []RichText{
			{
				PlainText: "Test DB",
			},
		},
	}

	mock := &MockNotionAPI{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := &discoverer{client: mock, logger: logger}

	title := discoverer.extractDatabaseTitle(db)
	if title != "Test DB" {
		t.Errorf("Expected title 'Test DB', got %q", title)
	}
}

func TestDiscoverer_ExtractDatabaseTitle_Fallback(t *testing.T) {
	db := &Database{
		ID:    "db1",
		URL:   "https://notion.so/db1",
		Title: nil,
	}

	mock := &MockNotionAPI{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := &discoverer{client: mock, logger: logger}

	title := discoverer.extractDatabaseTitle(db)
	if title != "https://notion.so/db1" {
		t.Errorf("Expected fallback URL, got %q", title)
	}
}

func TestDiscoverer_CyclePrevention(t *testing.T) {
	// This test verifies that the same page ID is not discovered twice
	mock := &MockNotionAPI{
		searchPagesFunc: func(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
			page := &Page{
				ID:             "page1",
				Object:         "page",
				CreatedTime:    time.Now(),
				LastEditedTime: time.Now(),
				URL:            "https://notion.so/page1",
				Properties:     map[string]interface{}{},
			}

			return &SearchResult{
				Results: []interface{}{page},
				HasMore: false,
			}, nil
		},
		getBlockChildrenFunc: func(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
			return &BlockList{
				Results: []*Block{},
				HasMore: false,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	discoverer := NewDiscoverer(mock, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pageChan, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{})
	if err != nil {
		t.Fatalf("DiscoverPages failed: %v", err)
	}

	var pages []*PageInfo
	for page := range pageChan {
		pages = append(pages, page)
	}

	if len(pages) != 1 {
		t.Errorf("Expected 1 page (no duplicates), got %d", len(pages))
	}
}
