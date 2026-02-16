package notion

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wesm/msgvault/internal/store"
)

// skipIfNoToken skips the test if NOTION_INTEGRATION_TOKEN is not set.
func skipIfNoToken(t *testing.T) string {
	t.Helper()
	token := os.Getenv("NOTION_INTEGRATION_TOKEN")
	if token == "" {
		t.Skip("NOTION_INTEGRATION_TOKEN not set; skipping integration test")
	}
	return token
}

func newIntegrationClient(t *testing.T) *ConcreteClient {
	t.Helper()
	token := skipIfNoToken(t)
	rl := NewRateLimiter(DefaultTokensPerSec)
	client := NewClient(token, WithRateLimiter(rl))
	t.Cleanup(func() { client.Close() })
	return client
}

func TestIntegration_SearchPages(t *testing.T) {
	client := newIntegrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.SearchPages(ctx, &SearchOpts{
		Filter:   &SearchFilter{Property: "object", Value: "page"},
		PageSize: 5,
	})
	if err != nil {
		t.Fatalf("SearchPages failed: %v", err)
	}

	if len(result.Results) == 0 {
		t.Fatal("expected at least one page in search results")
	}

	t.Logf("Found %d pages (has_more=%v)", len(result.Results), result.HasMore)
}

func TestIntegration_GetPage(t *testing.T) {
	client := newIntegrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use NOTION_ID_NOTES if available, otherwise search for a page
	pageID := os.Getenv("NOTION_ID_NOTES")
	if pageID == "" {
		// Find a page to test with
		result, err := client.SearchPages(ctx, &SearchOpts{
			Filter:   &SearchFilter{Property: "object", Value: "page"},
			PageSize: 1,
		})
		if err != nil {
			t.Fatalf("SearchPages: %v", err)
		}
		if len(result.Results) == 0 {
			t.Skip("no pages found in workspace")
		}
		// Parse first result to get ID
		page, err := parseSearchResultPage(result.Results[0])
		if err != nil {
			t.Fatalf("parse page: %v", err)
		}
		pageID = page.ID
	}

	page, err := client.GetPage(ctx, pageID)
	if err != nil {
		t.Fatalf("GetPage(%s) failed: %v", pageID, err)
	}

	if page.ID == "" {
		t.Error("page ID is empty")
	}
	if page.Object != "page" {
		t.Errorf("expected object=page, got %q", page.Object)
	}

	t.Logf("Page: id=%s created=%s edited=%s url=%s",
		page.ID, page.CreatedTime.Format(time.RFC3339),
		page.LastEditedTime.Format(time.RFC3339), page.URL)
}

func TestIntegration_GetPageBlocks(t *testing.T) {
	client := newIntegrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pageID := os.Getenv("NOTION_ID_NOTES")
	if pageID == "" {
		t.Skip("NOTION_ID_NOTES not set")
	}

	blockList, err := client.GetBlockChildren(ctx, pageID, &BlockListOpts{
		PageSize: 100,
	})
	if err != nil {
		t.Fatalf("GetBlockChildren(%s) failed: %v", pageID, err)
	}

	if len(blockList.Results) == 0 {
		t.Log("Warning: page has no blocks")
	}

	// Log block types found
	typeCounts := make(map[string]int)
	for _, block := range blockList.Results {
		typeCounts[block.Type]++
	}
	t.Logf("Found %d blocks:", len(blockList.Results))
	for typ, count := range typeCounts {
		t.Logf("  %s: %d", typ, count)
	}
}

func TestIntegration_MarkdownConversion(t *testing.T) {
	client := newIntegrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pageID := os.Getenv("NOTION_ID_NOTES")
	if pageID == "" {
		t.Skip("NOTION_ID_NOTES not set")
	}

	// Fetch blocks
	fetcher := NewFetcher(client, nil)
	content, err := fetcher.FetchPageContent(ctx, pageID)
	if err != nil {
		t.Fatalf("FetchPageContent: %v", err)
	}

	if len(content.Blocks) == 0 {
		t.Skip("page has no blocks")
	}

	// Convert to markdown
	converter := NewConverter(nil)
	markdown, err := converter.ConvertToMarkdown(content.Blocks)
	if err != nil {
		t.Fatalf("ConvertToMarkdown: %v", err)
	}

	if markdown == "" {
		t.Error("expected non-empty markdown output")
	}

	// Basic quality checks
	if len(markdown) < 10 {
		t.Errorf("markdown suspiciously short: %d bytes", len(markdown))
	}

	t.Logf("Markdown output (%d bytes):\n%.500s", len(markdown), markdown)
}

func TestIntegration_Discovery(t *testing.T) {
	client := newIntegrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	discoverer := NewDiscoverer(client, nil)
	pagesCh, err := discoverer.DiscoverPages(ctx, &DiscoveryOpts{
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("DiscoverPages: %v", err)
	}

	var pages []*PageInfo
	for page := range pagesCh {
		pages = append(pages, page)
		t.Logf("Discovered: id=%s title=%q is_db=%v children=%d",
			page.ID, page.Title, page.IsDatabase, len(page.ChildPages))
	}

	if len(pages) == 0 {
		t.Fatal("expected at least one discovered page")
	}

	t.Logf("Total discovered: %d pages", len(pages))
}

func TestIntegration_FullSync(t *testing.T) {
	token := skipIfNoToken(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create in-memory store
	dbPath := filepath.Join(t.TempDir(), "integration_test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	if err := s.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	// Create full pipeline
	rl := NewRateLimiter(DefaultTokensPerSec)
	client := NewClient(token, WithRateLimiter(rl))
	defer client.Close()

	nstore := NewNotionStore(s, nil)

	opts := DefaultSyncOptions()
	opts.Limit = 3 // Small limit for testing

	syncer := NewSyncer(client, nstore, opts).
		WithProgress(&testProgress{t: t})

	result, err := syncer.Full(ctx, "integration-test")
	if err != nil {
		t.Fatalf("Full sync failed: %v", err)
	}

	t.Logf("Sync result: processed=%d added=%d skipped=%d errors=%d duration=%s",
		result.PagesProcessed, result.PagesAdded, result.PagesSkipped,
		result.Errors, result.Duration)

	if result.PagesAdded == 0 {
		t.Error("expected at least one page to be added")
	}

	// Verify data was stored correctly
	source, err := s.GetOrCreateSource("notion", "integration-test")
	if err != nil {
		t.Fatalf("get source: %v", err)
	}

	sources, err := s.ListSources("notion")
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(sources) == 0 {
		t.Error("expected at least one notion source")
	}

	// Verify sync cursor was set
	cursor, err := nstore.GetSyncCursor(source.ID)
	if err != nil {
		t.Fatalf("get cursor: %v", err)
	}
	if cursor.IsZero() {
		t.Error("expected sync cursor to be set after full sync")
	}
}

func TestIntegration_IncrementalSync(t *testing.T) {
	token := skipIfNoToken(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Create store
	dbPath := filepath.Join(t.TempDir(), "incr_test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	if err := s.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	rl := NewRateLimiter(DefaultTokensPerSec)
	client := NewClient(token, WithRateLimiter(rl))
	defer client.Close()

	nstore := NewNotionStore(s, nil)

	opts := DefaultSyncOptions()
	opts.Limit = 3

	syncer := NewSyncer(client, nstore, opts)

	// First: full sync
	result1, err := syncer.Full(ctx, "incr-test")
	if err != nil {
		t.Fatalf("full sync: %v", err)
	}
	t.Logf("Full sync: added=%d", result1.PagesAdded)

	// Then: incremental sync (should find nothing new if no changes were made)
	result2, err := syncer.Incremental(ctx, "incr-test")
	if err != nil {
		t.Fatalf("incremental sync: %v", err)
	}
	t.Logf("Incremental sync: processed=%d updated=%d", result2.PagesProcessed, result2.PagesUpdated)
}

func TestIntegration_MarkdownQuality(t *testing.T) {
	client := newIntegrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pageID := os.Getenv("NOTION_ID_NOTES")
	if pageID == "" {
		t.Skip("NOTION_ID_NOTES not set")
	}

	// Fetch and convert
	fetcher := NewFetcher(client, nil)
	content, err := fetcher.FetchPageContent(ctx, pageID)
	if err != nil {
		t.Fatalf("FetchPageContent: %v", err)
	}

	converter := NewConverter(nil)
	markdown, err := converter.ConvertToMarkdown(content.Blocks)
	if err != nil {
		t.Fatalf("ConvertToMarkdown: %v", err)
	}

	// Quality assertions
	if markdown == "" {
		t.Fatal("empty markdown output")
	}

	// Check for common markdown elements
	hasHeading := strings.Contains(markdown, "#")
	hasText := len(strings.TrimSpace(markdown)) > 20

	if !hasText {
		t.Error("markdown output has very little text content")
	}

	t.Logf("Quality check: has_heading=%v has_text=%v length=%d",
		hasHeading, hasText, len(markdown))

	// Ensure no raw HTML or JSON artifacts leaked through
	if strings.Contains(markdown, `"object":`) {
		t.Error("markdown contains raw JSON - conversion may have failed")
	}
	if strings.Contains(markdown, "<div>") {
		t.Error("markdown contains raw HTML div tags")
	}
}

// testProgress logs sync progress during integration tests.
type testProgress struct {
	t *testing.T
}

func (p *testProgress) OnPageStart(pageID, title string) {
	p.t.Logf("  Syncing: %s (%s)", title, pageID[:8])
}

func (p *testProgress) OnPageComplete(pageID string)    {}
func (p *testProgress) OnProgress(processed, total int) {}

func (p *testProgress) OnError(pageID string, err error) {
	p.t.Logf("  Error: %s: %v", pageID, err)
}
