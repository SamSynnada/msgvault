package notion

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/wesm/msgvault/internal/store"
)

// mockFetcher implements the Fetcher interface for testing.
type mockFetcher struct {
	fetchPageContentFn  func(ctx context.Context, pageID string) (*PageContent, error)
	fetchDatabaseRowsFn func(ctx context.Context, dbID string, opts *QueryOpts) ([]*DatabaseRow, error)
}

func (m *mockFetcher) FetchPageContent(ctx context.Context, pageID string) (*PageContent, error) {
	if m.fetchPageContentFn != nil {
		return m.fetchPageContentFn(ctx, pageID)
	}
	return &PageContent{PageID: pageID, Blocks: []*Block{}}, nil
}

func (m *mockFetcher) FetchDatabaseRows(ctx context.Context, dbID string, opts *QueryOpts) ([]*DatabaseRow, error) {
	if m.fetchDatabaseRowsFn != nil {
		return m.fetchDatabaseRowsFn(ctx, dbID, opts)
	}
	return nil, nil
}

// mockConverter implements the Converter interface for testing.
type mockConverter struct {
	convertFn func(blocks []*Block) (string, error)
}

func (m *mockConverter) ConvertToMarkdown(blocks []*Block) (string, error) {
	if m.convertFn != nil {
		return m.convertFn(blocks)
	}
	return "# Mock Markdown", nil
}

func (m *mockConverter) ConvertBlockToMarkdown(block *Block) (string, error) {
	return "mock block", nil
}

// mockDiscoverer implements the Discoverer interface for testing.
type mockDiscoverer struct {
	pages []*PageInfo
	err   error
}

func (m *mockDiscoverer) DiscoverPages(ctx context.Context, opts *DiscoveryOpts) (<-chan *PageInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan *PageInfo, len(m.pages))
	go func() {
		defer close(ch)
		for _, p := range m.pages {
			select {
			case <-ctx.Done():
				return
			case ch <- p:
			}
		}
	}()
	return ch, nil
}

// mockSyncProgress captures progress events for testing.
type mockSyncProgress struct {
	started   []string
	completed []string
	errors    []string
}

func (m *mockSyncProgress) OnPageStart(pageID, title string) {
	m.started = append(m.started, pageID)
}

func (m *mockSyncProgress) OnPageComplete(pageID string) {
	m.completed = append(m.completed, pageID)
}

func (m *mockSyncProgress) OnProgress(processed, total int) {}

func (m *mockSyncProgress) OnError(pageID string, err error) {
	m.errors = append(m.errors, pageID)
}

func setupSyncerTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := s.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSyncer_Full_EmptyWorkspace(t *testing.T) {
	s := setupSyncerTestStore(t)
	ns := NewNotionStore(s, nil)

	syncer := NewSyncer(nil, ns, nil)
	syncer.discoverer = &mockDiscoverer{pages: nil}

	result, err := syncer.Full(context.Background(), "test-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PagesProcessed != 0 {
		t.Errorf("expected 0 pages processed, got %d", result.PagesProcessed)
	}
	if result.PagesAdded != 0 {
		t.Errorf("expected 0 pages added, got %d", result.PagesAdded)
	}
}

func TestSyncer_Full_SinglePage(t *testing.T) {
	s := setupSyncerTestStore(t)
	ns := NewNotionStore(s, nil)

	now := time.Now().Truncate(time.Second)
	pages := []*PageInfo{
		{
			ID:             "page-001",
			Title:          "Test Page",
			CreatedTime:    now.Add(-time.Hour),
			LastEditedTime: now,
		},
	}

	syncer := NewSyncer(nil, ns, nil)
	syncer.discoverer = &mockDiscoverer{pages: pages}
	syncer.fetcher = &mockFetcher{
		fetchPageContentFn: func(ctx context.Context, pageID string) (*PageContent, error) {
			return &PageContent{
				PageID: pageID,
				Blocks: []*Block{
					{
						Type: "paragraph",
						Paragraph: &ParagraphBlock{
							RichText: []RichText{
								{PlainText: "Hello from Notion"},
							},
						},
					},
				},
			}, nil
		},
	}
	syncer.converter = &mockConverter{
		convertFn: func(blocks []*Block) (string, error) {
			return "Hello from Notion", nil
		},
	}

	result, err := syncer.Full(context.Background(), "test-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PagesProcessed != 1 {
		t.Errorf("expected 1 page processed, got %d", result.PagesProcessed)
	}
	if result.PagesAdded != 1 {
		t.Errorf("expected 1 page added, got %d", result.PagesAdded)
	}

	// Verify the page was stored
	source, err := s.GetOrCreateSource("notion", "test-workspace")
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	existing, err := s.MessageExistsBatch(source.ID, []string{"page-001"})
	if err != nil {
		t.Fatalf("check existing: %v", err)
	}
	if len(existing) != 1 {
		t.Errorf("expected 1 existing message, got %d", len(existing))
	}
}

func TestSyncer_Full_SkipExisting(t *testing.T) {
	s := setupSyncerTestStore(t)
	ns := NewNotionStore(s, nil)

	now := time.Now().Truncate(time.Second)
	pages := []*PageInfo{
		{
			ID:             "page-001",
			Title:          "Existing Page",
			CreatedTime:    now.Add(-time.Hour),
			LastEditedTime: now,
		},
	}

	syncer := NewSyncer(nil, ns, nil)
	syncer.discoverer = &mockDiscoverer{pages: pages}
	syncer.fetcher = &mockFetcher{
		fetchPageContentFn: func(ctx context.Context, pageID string) (*PageContent, error) {
			return &PageContent{PageID: pageID, Blocks: []*Block{}}, nil
		},
	}
	syncer.converter = &mockConverter{}

	// First sync
	result1, err := syncer.Full(context.Background(), "test-workspace")
	if err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if result1.PagesAdded != 1 {
		t.Errorf("first sync: expected 1 added, got %d", result1.PagesAdded)
	}

	// Second sync - should skip the existing page
	result2, err := syncer.Full(context.Background(), "test-workspace")
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if result2.PagesSkipped != 1 {
		t.Errorf("second sync: expected 1 skipped, got %d", result2.PagesSkipped)
	}
	if result2.PagesAdded != 0 {
		t.Errorf("second sync: expected 0 added, got %d", result2.PagesAdded)
	}
}

func TestSyncer_Full_ErrorRecovery(t *testing.T) {
	s := setupSyncerTestStore(t)
	ns := NewNotionStore(s, nil)

	now := time.Now().Truncate(time.Second)
	pages := []*PageInfo{
		{ID: "page-ok", Title: "Good Page", CreatedTime: now, LastEditedTime: now},
		{ID: "page-bad", Title: "Bad Page", CreatedTime: now, LastEditedTime: now},
		{ID: "page-ok2", Title: "Good Page 2", CreatedTime: now, LastEditedTime: now},
	}

	syncer := NewSyncer(nil, ns, nil)
	syncer.discoverer = &mockDiscoverer{pages: pages}
	syncer.fetcher = &mockFetcher{
		fetchPageContentFn: func(ctx context.Context, pageID string) (*PageContent, error) {
			if pageID == "page-bad" {
				return nil, errors.New("fetch failed")
			}
			return &PageContent{PageID: pageID, Blocks: []*Block{}}, nil
		},
	}
	syncer.converter = &mockConverter{}

	progress := &mockSyncProgress{}
	syncer.WithProgress(progress)

	result, err := syncer.Full(context.Background(), "test-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should process all 3, add 2, error on 1
	if result.PagesProcessed != 3 {
		t.Errorf("expected 3 processed, got %d", result.PagesProcessed)
	}
	if result.PagesAdded != 2 {
		t.Errorf("expected 2 added, got %d", result.PagesAdded)
	}
	if result.Errors != 1 {
		t.Errorf("expected 1 error, got %d", result.Errors)
	}
}

func TestSyncer_Full_ContextCancellation(t *testing.T) {
	s := setupSyncerTestStore(t)
	ns := NewNotionStore(s, nil)

	ctx, cancel := context.WithCancel(context.Background())

	now := time.Now().Truncate(time.Second)
	pages := []*PageInfo{
		{ID: "page-1", Title: "Page 1", CreatedTime: now, LastEditedTime: now},
		{ID: "page-2", Title: "Page 2", CreatedTime: now, LastEditedTime: now},
	}

	syncer := NewSyncer(nil, ns, nil)
	syncer.discoverer = &mockDiscoverer{pages: pages}
	syncer.fetcher = &mockFetcher{
		fetchPageContentFn: func(ctx context.Context, pageID string) (*PageContent, error) {
			if pageID == "page-1" {
				cancel() // Cancel after first page
			}
			return &PageContent{PageID: pageID, Blocks: []*Block{}}, nil
		},
	}
	syncer.converter = &mockConverter{}

	_, err := syncer.Full(ctx, "test-workspace")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestSyncer_Full_SyncCursorUpdated(t *testing.T) {
	s := setupSyncerTestStore(t)
	ns := NewNotionStore(s, nil)

	now := time.Now().Truncate(time.Second)
	pages := []*PageInfo{
		{ID: "page-1", Title: "Page", CreatedTime: now.Add(-time.Hour), LastEditedTime: now},
	}

	syncer := NewSyncer(nil, ns, nil)
	syncer.discoverer = &mockDiscoverer{pages: pages}
	syncer.fetcher = &mockFetcher{
		fetchPageContentFn: func(ctx context.Context, pageID string) (*PageContent, error) {
			return &PageContent{PageID: pageID, Blocks: []*Block{}}, nil
		},
	}
	syncer.converter = &mockConverter{}

	_, err := syncer.Full(context.Background(), "test-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sync cursor was set
	source, err := s.GetOrCreateSource("notion", "test-workspace")
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	cursor, err := ns.GetSyncCursor(source.ID)
	if err != nil {
		t.Fatalf("get cursor: %v", err)
	}
	if cursor.IsZero() {
		t.Error("expected sync cursor to be set")
	}
}

func TestSyncer_Full_DiscoveryError(t *testing.T) {
	s := setupSyncerTestStore(t)
	ns := NewNotionStore(s, nil)

	syncer := NewSyncer(nil, ns, nil)
	syncer.discoverer = &mockDiscoverer{err: errors.New("discovery failed")}

	_, err := syncer.Full(context.Background(), "test-workspace")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errors.New("discovery failed")) {
		// Just check error message contains our text
		if err.Error() != "discover pages: discovery failed" {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestSyncer_Full_ProgressReporting(t *testing.T) {
	s := setupSyncerTestStore(t)
	ns := NewNotionStore(s, nil)

	now := time.Now().Truncate(time.Second)
	pages := []*PageInfo{
		{ID: "p1", Title: "Page 1", CreatedTime: now, LastEditedTime: now},
		{ID: "p2", Title: "Page 2", CreatedTime: now, LastEditedTime: now},
	}

	progress := &mockSyncProgress{}

	syncer := NewSyncer(nil, ns, nil)
	syncer.discoverer = &mockDiscoverer{pages: pages}
	syncer.fetcher = &mockFetcher{
		fetchPageContentFn: func(ctx context.Context, pageID string) (*PageContent, error) {
			return &PageContent{PageID: pageID, Blocks: []*Block{}}, nil
		},
	}
	syncer.converter = &mockConverter{}
	syncer.progress = progress

	_, err := syncer.Full(context.Background(), "test-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(progress.started) != 2 {
		t.Errorf("expected 2 started events, got %d", len(progress.started))
	}
	if len(progress.completed) != 2 {
		t.Errorf("expected 2 completed events, got %d", len(progress.completed))
	}
}

func TestSyncer_syncPage(t *testing.T) {
	s := setupSyncerTestStore(t)
	ns := NewNotionStore(s, nil)

	source, err := s.GetOrCreateSource("notion", "test-workspace")
	if err != nil {
		t.Fatalf("get source: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	page := &PageInfo{
		ID:             "sync-page-1",
		Title:          "Sync Test",
		CreatedTime:    now.Add(-time.Hour),
		LastEditedTime: now,
	}

	syncer := NewSyncer(nil, ns, nil)
	syncer.fetcher = &mockFetcher{
		fetchPageContentFn: func(ctx context.Context, pageID string) (*PageContent, error) {
			return &PageContent{
				PageID: pageID,
				Blocks: []*Block{
					{Type: "heading_1", Heading1: &Heading1Block{RichText: []RichText{{PlainText: "Title"}}}},
				},
			}, nil
		},
	}
	syncer.converter = &mockConverter{
		convertFn: func(blocks []*Block) (string, error) {
			return "# Title", nil
		},
	}

	err = syncer.syncPage(context.Background(), source.ID, page)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify stored message
	existing, err := s.MessageExistsBatch(source.ID, []string{"sync-page-1"})
	if err != nil {
		t.Fatalf("check existing: %v", err)
	}
	if len(existing) != 1 {
		t.Errorf("expected 1 stored message, got %d", len(existing))
	}

	// Verify raw blocks stored as JSON
	msgID := existing["sync-page-1"]
	rawData, err := s.GetMessageRaw(msgID)
	if err != nil {
		t.Fatalf("get raw: %v", err)
	}
	var blocks []*Block
	if err := json.Unmarshal(rawData, &blocks); err != nil {
		t.Fatalf("unmarshal raw blocks: %v", err)
	}
	if len(blocks) != 1 {
		t.Errorf("expected 1 raw block, got %d", len(blocks))
	}
}

func TestPageToPageInfo(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	page := &Page{
		ID:             "test-id",
		CreatedTime:    now.Add(-time.Hour),
		LastEditedTime: now,
		URL:            "https://www.notion.so/Test-Page-abc123",
		Properties: map[string]interface{}{
			"title": map[string]interface{}{
				"type": "title",
				"title": []interface{}{
					map[string]interface{}{
						"plain_text": "My Page Title",
					},
				},
			},
		},
	}

	info := pageToPageInfo(page)
	if info.ID != "test-id" {
		t.Errorf("expected ID test-id, got %s", info.ID)
	}
	if info.Title != "My Page Title" {
		t.Errorf("expected title 'My Page Title', got %q", info.Title)
	}
	if !info.CreatedTime.Equal(now.Add(-time.Hour)) {
		t.Errorf("expected created time %v, got %v", now.Add(-time.Hour), info.CreatedTime)
	}
}

func TestExtractPageTitle_Fallback(t *testing.T) {
	page := &Page{
		ID:  "no-title",
		URL: "https://www.notion.so/No-Title-abc123",
	}

	title := extractPageTitle(page)
	if title != page.URL {
		t.Errorf("expected URL fallback, got %q", title)
	}
}

func TestDefaultSyncOptions(t *testing.T) {
	opts := DefaultSyncOptions()
	if opts.CheckpointInterval != 50 {
		t.Errorf("expected checkpoint interval 50, got %d", opts.CheckpointInterval)
	}
	if opts.Limit != 0 {
		t.Errorf("expected limit 0, got %d", opts.Limit)
	}
}
