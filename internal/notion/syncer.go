package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/wesm/msgvault/internal/store"
)

// SyncOptions configures sync behavior.
type SyncOptions struct {
	// Limit caps the number of pages to sync (0 = unlimited).
	Limit int

	// CheckpointInterval is how often to save progress (default: every 50 pages).
	CheckpointInterval int

	// NoResume forces a fresh sync even if a checkpoint exists.
	NoResume bool
}

// DefaultSyncOptions returns sensible defaults.
func DefaultSyncOptions() *SyncOptions {
	return &SyncOptions{
		CheckpointInterval: 50,
	}
}

// SyncResult holds the summary of a sync operation.
type SyncResult struct {
	PagesProcessed int
	PagesAdded     int
	PagesUpdated   int
	PagesSkipped   int
	Errors         int
	Duration       time.Duration
	WasResumed     bool
}

// SyncProgress reports sync progress to the caller.
type SyncProgress interface {
	OnPageStart(pageID, title string)
	OnPageComplete(pageID string)
	OnProgress(processed, total int)
	OnError(pageID string, err error)
}

// NullProgress is a no-op progress reporter.
type NullProgress struct{}

func (NullProgress) OnPageStart(string, string) {}
func (NullProgress) OnPageComplete(string)      {}
func (NullProgress) OnProgress(int, int)        {}
func (NullProgress) OnError(string, error)      {}

// Syncer orchestrates Notion page synchronization.
type Syncer struct {
	client     NotionAPI
	nstore     *NotionStore
	fetcher    Fetcher
	converter  Converter
	discoverer Discoverer
	logger     *slog.Logger
	progress   SyncProgress
	opts       *SyncOptions
}

// NewSyncer creates a new Syncer.
func NewSyncer(client NotionAPI, nstore *NotionStore, opts *SyncOptions) *Syncer {
	if opts == nil {
		opts = DefaultSyncOptions()
	}
	s := &Syncer{
		client:    client,
		nstore:    nstore,
		converter: NewConverter(nil),
		logger:    slog.Default(),
		progress:  NullProgress{},
		opts:      opts,
	}
	// Only create fetcher/discoverer when client is provided.
	// Tests may pass nil client and override these fields directly.
	if client != nil {
		s.fetcher = NewFetcher(client, nil)
		s.discoverer = NewDiscoverer(client, nil)
	}
	return s
}

// WithLogger sets the logger.
func (s *Syncer) WithLogger(logger *slog.Logger) *Syncer {
	s.logger = logger
	s.converter = NewConverter(logger)
	if s.client != nil {
		s.fetcher = NewFetcher(s.client, logger)
		s.discoverer = NewDiscoverer(s.client, logger)
	}
	return s
}

// WithProgress sets the progress reporter.
func (s *Syncer) WithProgress(p SyncProgress) *Syncer {
	s.progress = p
	return s
}

// Full performs a full sync of all workspace pages.
func (s *Syncer) Full(ctx context.Context, workspaceID string) (result *SyncResult, err error) {
	startTime := time.Now()
	result = &SyncResult{}

	// Get or create source
	source, err := s.nstore.GetOrCreateSource(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get/create source: %w", err)
	}

	// Start sync run
	syncID, err := s.nstore.Store().StartSync(source.ID, "notion_full")
	if err != nil {
		return nil, fmt.Errorf("start sync: %w", err)
	}

	// Defer failure handling — recover from panics
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			s.logger.Error("sync panic recovered", "panic", r, "stack", string(stack))
			if failErr := s.nstore.Store().FailSync(syncID, fmt.Sprintf("panic: %v", r)); failErr != nil {
				s.logger.Error("failed to record sync failure", "error", failErr)
			}
			result = nil
			err = fmt.Errorf("sync panicked: %v", r)
		}
	}()

	s.logger.Info("starting notion full sync", "workspace", workspaceID)

	// Discover pages
	// MaxDepth=1 avoids recursive block traversal — search already returns all pages.
	discoveryOpts := &DiscoveryOpts{
		Limit:    s.opts.Limit,
		MaxDepth: 1,
	}
	pagesCh, err := s.discoverer.DiscoverPages(ctx, discoveryOpts)
	if err != nil {
		_ = s.nstore.Store().FailSync(syncID, err.Error())
		return nil, fmt.Errorf("discover pages: %w", err)
	}

	// Check which pages already exist
	var latestEditedTime time.Time
	checkpoint := &store.Checkpoint{}

	for page := range pagesCh {
		if ctx.Err() != nil {
			break
		}

		result.PagesProcessed++
		s.progress.OnPageStart(page.ID, page.Title)

		// Check if page already exists
		existing, err := s.nstore.Store().MessageExistsBatch(source.ID, []string{page.ID})
		if err != nil {
			s.logger.Warn("failed to check existing page", "page_id", page.ID, "error", err)
			result.Errors++
			s.progress.OnError(page.ID, err)
			continue
		}

		if _, exists := existing[page.ID]; exists {
			result.PagesSkipped++
			s.progress.OnPageComplete(page.ID)
			continue
		}

		// Fetch page content
		if err := s.syncPage(ctx, source.ID, page); err != nil {
			s.logger.Warn("failed to sync page", "page_id", page.ID, "title", page.Title, "error", err)
			result.Errors++
			checkpoint.ErrorsCount++
			s.progress.OnError(page.ID, err)
			continue
		}

		result.PagesAdded++
		checkpoint.MessagesAdded++
		s.progress.OnPageComplete(page.ID)

		// Track latest edited time for cursor
		if page.LastEditedTime.After(latestEditedTime) {
			latestEditedTime = page.LastEditedTime
		}

		// Checkpoint periodically
		checkpoint.MessagesProcessed = int64(result.PagesProcessed)
		if s.opts.CheckpointInterval > 0 && result.PagesProcessed%s.opts.CheckpointInterval == 0 {
			if err := s.nstore.Store().UpdateSyncCheckpoint(syncID, checkpoint); err != nil {
				s.logger.Warn("failed to save checkpoint", "error", err)
			}
		}

		s.progress.OnProgress(result.PagesProcessed, 0)
	}

	// Check for context cancellation
	if ctx.Err() != nil {
		_ = s.nstore.Store().FailSync(syncID, "cancelled")
		return result, ctx.Err()
	}

	// Update sync cursor
	if !latestEditedTime.IsZero() {
		if err := s.nstore.UpdateSyncCursor(source.ID, latestEditedTime); err != nil {
			s.logger.Warn("failed to update sync cursor", "error", err)
		}
	}

	// Complete sync
	cursorStr := ""
	if !latestEditedTime.IsZero() {
		cursorStr = latestEditedTime.Format(time.RFC3339)
	}
	if err := s.nstore.Store().CompleteSync(syncID, cursorStr); err != nil {
		s.logger.Warn("failed to complete sync", "error", err)
	}

	result.Duration = time.Since(startTime)
	s.logger.Info("notion sync completed",
		"workspace", workspaceID,
		"pages_processed", result.PagesProcessed,
		"pages_added", result.PagesAdded,
		"pages_skipped", result.PagesSkipped,
		"errors", result.Errors,
		"duration", result.Duration,
	)

	return result, nil
}

// Incremental syncs only pages modified since the last sync.
func (s *Syncer) Incremental(ctx context.Context, workspaceID string) (result *SyncResult, err error) {
	startTime := time.Now()
	result = &SyncResult{}

	// Get source
	source, err := s.nstore.GetOrCreateSource(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get/create source: %w", err)
	}

	// Get last sync cursor
	lastSync, err := s.nstore.GetSyncCursor(source.ID)
	if err != nil {
		return nil, fmt.Errorf("get sync cursor: %w", err)
	}
	if lastSync.IsZero() {
		s.logger.Info("no previous sync found, falling back to full sync")
		return s.Full(ctx, workspaceID)
	}

	// Start sync run
	syncID, err := s.nstore.Store().StartSync(source.ID, "notion_incremental")
	if err != nil {
		return nil, fmt.Errorf("start sync: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			s.logger.Error("sync panic recovered", "panic", r, "stack", string(stack))
			_ = s.nstore.Store().FailSync(syncID, fmt.Sprintf("panic: %v", r))
			result = nil
			err = fmt.Errorf("sync panicked: %v", r)
		}
	}()

	s.logger.Info("starting notion incremental sync",
		"workspace", workspaceID,
		"since", lastSync.Format(time.RFC3339),
	)

	// Search for pages modified since last sync
	var latestEditedTime time.Time
	var searchCursor string

	for {
		if ctx.Err() != nil {
			break
		}

		opts := &SearchOpts{
			Filter:   &SearchFilter{Property: "object", Value: "page"},
			Sort:     &SearchSort{Direction: "descending", Timestamp: "last_edited_time"},
			PageSize: 100,
		}
		if searchCursor != "" {
			opts.StartCursor = searchCursor
		}

		searchResult, err := s.client.SearchPages(ctx, opts)
		if err != nil {
			_ = s.nstore.Store().FailSync(syncID, err.Error())
			return nil, fmt.Errorf("search pages: %w", err)
		}

		// Process pages that were modified after our cursor
		reachedOldPages := false
		for _, item := range searchResult.Results {
			// Parse search result item to Page
			page, err := parseSearchResultPage(item)
			if err != nil {
				s.logger.Debug("skipping non-page search result", "error", err)
				continue
			}

			if page.LastEditedTime.Before(lastSync) || page.LastEditedTime.Equal(lastSync) {
				// Pages are sorted descending by last_edited_time,
				// so once we find one older than our cursor, we're done.
				reachedOldPages = true
				break
			}

			result.PagesProcessed++
			pageInfo := pageToPageInfo(page)

			s.progress.OnPageStart(pageInfo.ID, pageInfo.Title)

			if err := s.syncPage(ctx, source.ID, pageInfo); err != nil {
				s.logger.Warn("failed to sync page", "page_id", pageInfo.ID, "error", err)
				result.Errors++
				s.progress.OnError(pageInfo.ID, err)
				continue
			}

			result.PagesUpdated++
			s.progress.OnPageComplete(pageInfo.ID)

			if page.LastEditedTime.After(latestEditedTime) {
				latestEditedTime = page.LastEditedTime
			}
		}

		if reachedOldPages || !searchResult.HasMore || searchResult.NextCursor == nil {
			break
		}
		searchCursor = *searchResult.NextCursor
	}
	// Update sync cursor
	if !latestEditedTime.IsZero() {
		if err := s.nstore.UpdateSyncCursor(source.ID, latestEditedTime); err != nil {
			s.logger.Warn("failed to update sync cursor", "error", err)
		}
	}

	cursorStr := ""
	if !latestEditedTime.IsZero() {
		cursorStr = latestEditedTime.Format(time.RFC3339)
	}
	if err := s.nstore.Store().CompleteSync(syncID, cursorStr); err != nil {
		s.logger.Warn("failed to complete sync", "error", err)
	}

	result.Duration = time.Since(startTime)
	s.logger.Info("notion incremental sync completed",
		"workspace", workspaceID,
		"pages_processed", result.PagesProcessed,
		"pages_updated", result.PagesUpdated,
		"errors", result.Errors,
		"duration", result.Duration,
	)

	return result, nil
}

// syncPage fetches a single page's content, converts to markdown, and stores it.
func (s *Syncer) syncPage(ctx context.Context, sourceID int64, page *PageInfo) error {
	// Fetch blocks
	content, err := s.fetcher.FetchPageContent(ctx, page.ID)
	if err != nil {
		return fmt.Errorf("fetch content: %w", err)
	}

	// Convert to markdown
	markdown, err := s.converter.ConvertToMarkdown(content.Blocks)
	if err != nil {
		return fmt.Errorf("convert to markdown: %w", err)
	}

	// Serialize raw blocks as JSON for storage
	rawBlocks, err := json.Marshal(content.Blocks)
	if err != nil {
		return fmt.Errorf("marshal blocks: %w", err)
	}

	// Store
	_, err = s.nstore.UpsertNotionPage(sourceID, page, markdown, rawBlocks)
	if err != nil {
		return fmt.Errorf("store page: %w", err)
	}

	return nil
}

// pageToPageInfo converts a Page (API response) to a PageInfo (discovery type).
func pageToPageInfo(page *Page) *PageInfo {
	title := extractPageTitle(page)
	return &PageInfo{
		ID:             page.ID,
		Title:          title,
		CreatedTime:    page.CreatedTime,
		LastEditedTime: page.LastEditedTime,
	}
}

// parseSearchResultPage converts a search result item (interface{}) to a Page.
func parseSearchResultPage(item interface{}) (*Page, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("marshal item: %w", err)
	}
	var page Page
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("unmarshal page: %w", err)
	}
	if page.ID == "" || page.Object != "page" {
		return nil, fmt.Errorf("not a page object")
	}
	return &page, nil
}

// extractPageTitle extracts the title from a Page's properties.
func extractPageTitle(page *Page) string {
	props := page.Properties
	if props == nil {
		return page.URL
	}

	// Look for "title" property type
	for _, v := range props {
		propMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		propType, _ := propMap["type"].(string)
		if propType != "title" {
			continue
		}
		titleArr, ok := propMap["title"].([]interface{})
		if !ok || len(titleArr) == 0 {
			continue
		}
		firstItem, ok := titleArr[0].(map[string]interface{})
		if !ok {
			continue
		}
		if plainText, ok := firstItem["plain_text"].(string); ok && plainText != "" {
			return plainText
		}
	}

	return page.URL
}
