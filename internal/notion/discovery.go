// Package notion provides a discovery service for finding all pages and databases in a Notion workspace.
package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Discoverer discovers all pages and databases in a Notion workspace.
type Discoverer interface {
	DiscoverPages(ctx context.Context, opts *DiscoveryOpts) (<-chan *PageInfo, error)
}

// PageInfo represents metadata about a discovered Notion page.
type PageInfo struct {
	ID             string
	Title          string
	CreatedTime    time.Time
	LastEditedTime time.Time
	IsDatabase     bool
	ChildDatabases []*DatabaseInfo
	ChildPages     []*PageInfo
}

// DatabaseInfo represents metadata about a discovered database.
type DatabaseInfo struct {
	ID    string
	Title string
}

// DiscoveryOpts configures page discovery behavior.
type DiscoveryOpts struct {
	MaxDepth      int  // Max recursion depth (0 = unlimited)
	SkipDatabases bool // Skip discovering databases
	Limit         int  // Max pages to discover (0 = unlimited)
}

// discoverer implements the Discoverer interface.
type discoverer struct {
	client NotionAPI
	logger *slog.Logger
}

// NewDiscoverer creates a new page discoverer.
func NewDiscoverer(client NotionAPI, logger *slog.Logger) Discoverer {
	if logger == nil {
		logger = slog.Default()
	}
	return &discoverer{
		client: client,
		logger: logger,
	}
}

// DiscoverPages discovers all pages and databases in the workspace.
// It returns a channel that receives PageInfo as they are discovered.
// The channel is closed when discovery completes or the context is cancelled.
// Results are sent immediately as discovered (streaming).
func (d *discoverer) DiscoverPages(ctx context.Context, opts *DiscoveryOpts) (<-chan *PageInfo, error) {
	if opts == nil {
		opts = &DiscoveryOpts{}
	}

	// Validate options
	if opts.Limit < 0 {
		opts.Limit = 0
	}
	if opts.MaxDepth < 0 {
		opts.MaxDepth = 0
	}

	// Create output channel
	outChan := make(chan *PageInfo, 10) // Buffered to avoid blocking on sends

	// Launch discovery in a goroutine
	go func() {
		defer close(outChan)

		// Track discovered IDs to prevent cycles
		discovered := make(map[string]bool)
		count := 0
		countMu := sync.Mutex{}

		// Walk the page hierarchy starting from workspace root
		d.discoverPagesRecursive(ctx, outChan, opts, "", 0, discovered, &count, &countMu)
	}()

	return outChan, nil
}

// discoverPagesRecursive recursively discovers pages starting from a parent.
// parentID is empty for workspace root discovery.
func (d *discoverer) discoverPagesRecursive(
	ctx context.Context,
	outChan chan<- *PageInfo,
	opts *DiscoveryOpts,
	parentID string,
	depth int,
	discovered map[string]bool,
	count *int,
	countMu *sync.Mutex,
) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Check depth limit
	if opts.MaxDepth > 0 && depth >= opts.MaxDepth {
		return
	}

	// Search for pages at this level
	cursor := ""
	for {
		// Check context again
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check page limit before making request
		countMu.Lock()
		if opts.Limit > 0 && *count >= opts.Limit {
			countMu.Unlock()
			return
		}
		countMu.Unlock()

		// Search for pages
		searchOpts := &SearchOpts{
			PageSize:    100, // Max page size
			StartCursor: cursor,
		}

		// When at root level, search all pages in workspace
		// When at a child level, we search by drilling down blocks
		if parentID == "" {
			searchOpts.Filter = &SearchFilter{
				Property: "object",
				Value:    "page",
			}
		}

		result, err := d.client.SearchPages(ctx, searchOpts)
		if err != nil {
			d.logger.Error("search pages failed", "error", err, "cursor", cursor, "parent_id", parentID)
			return
		}

		// Process search results
		for _, item := range result.Results {
			// Parse the item (can be Page or Database)
			pageInfo, isDB, err := d.parseSearchResult(item, discovered)
			if err != nil {
				d.logger.Debug("failed to parse search result", "error", err)
				continue
			}

			if pageInfo == nil {
				continue
			}

			// Check limit
			countMu.Lock()
			if opts.Limit > 0 && *count >= opts.Limit {
				countMu.Unlock()
				return
			}
			*count++
			countMu.Unlock()

			// Mark as discovered
			discovered[pageInfo.ID] = true

			// Discover child blocks (child pages and databases)
			d.discoverChildBlocks(ctx, pageInfo, opts, depth, discovered)

			// Send the page info to the channel
			select {
			case outChan <- pageInfo:
			case <-ctx.Done():
				return
			}

			// If this is a database and we're not skipping databases, we might discover it
			if isDB && !opts.SkipDatabases {
				// Database already marked as discovered in pageInfo
			}
		}

		// Check for more results
		if !result.HasMore || result.NextCursor == nil {
			break
		}

		cursor = *result.NextCursor
	}

	// If at root level, also discover top-level pages by searching for them
	// This catches pages that might not appear in the generic search
	if parentID == "" && depth == 0 {
		d.discoverTopLevelPagesViaSearch(ctx, outChan, opts, discovered, count, countMu)
	}
}

// discoverTopLevelPagesViaSearch searches for all top-level pages in the workspace.
func (d *discoverer) discoverTopLevelPagesViaSearch(
	ctx context.Context,
	outChan chan<- *PageInfo,
	opts *DiscoveryOpts,
	discovered map[string]bool,
	count *int,
	countMu *sync.Mutex,
) {
	cursor := ""
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		countMu.Lock()
		if opts.Limit > 0 && *count >= opts.Limit {
			countMu.Unlock()
			return
		}
		countMu.Unlock()

		// Do a generic search for all items
		searchOpts := &SearchOpts{
			PageSize:    100,
			StartCursor: cursor,
		}

		result, err := d.client.SearchPages(ctx, searchOpts)
		if err != nil {
			d.logger.Debug("generic search failed", "error", err)
			return
		}

		for _, item := range result.Results {
			pageInfo, _, err := d.parseSearchResult(item, discovered)
			if err != nil {
				d.logger.Debug("failed to parse search result", "error", err)
				continue
			}

			if pageInfo == nil {
				continue
			}

			// Skip if already discovered
			if discovered[pageInfo.ID] {
				continue
			}

			countMu.Lock()
			if opts.Limit > 0 && *count >= opts.Limit {
				countMu.Unlock()
				return
			}
			*count++
			countMu.Unlock()

			discovered[pageInfo.ID] = true

			// Discover child blocks
			d.discoverChildBlocks(ctx, pageInfo, opts, 0, discovered)

			// Send the page info
			select {
			case outChan <- pageInfo:
			case <-ctx.Done():
				return
			}
		}

		if !result.HasMore || result.NextCursor == nil {
			break
		}

		cursor = *result.NextCursor
	}
}

// discoverChildBlocks discovers child pages and databases within a page's blocks.
func (d *discoverer) discoverChildBlocks(
	ctx context.Context,
	pageInfo *PageInfo,
	opts *DiscoveryOpts,
	currentDepth int,
	discovered map[string]bool,
) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Check depth limit for recursion
	if opts.MaxDepth > 0 && currentDepth+1 >= opts.MaxDepth {
		return
	}

	// Get all blocks for this page
	cursor := ""
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		blockListOpts := &BlockListOpts{
			PageSize:    100,
			StartCursor: cursor,
		}

		blockList, err := d.client.GetBlockChildren(ctx, pageInfo.ID, blockListOpts)
		if err != nil {
			d.logger.Debug("get block children failed", "page_id", pageInfo.ID, "error", err)
			return
		}

		// Process blocks looking for child_database and child_page
		for _, block := range blockList.Results {
			if block.ChildDatabase != nil {
				// Found a child database block
				dbInfo := &DatabaseInfo{
					ID:    block.ID,
					Title: block.ChildDatabase.Title,
				}
				pageInfo.ChildDatabases = append(pageInfo.ChildDatabases, dbInfo)
			}

			if block.ChildPage != nil {
				// Found a child page block
				childPageInfo := &PageInfo{
					ID:             block.ID,
					Title:          block.ChildPage.Title,
					CreatedTime:    block.CreatedTime,
					LastEditedTime: block.LastEditedTime,
					IsDatabase:     false,
				}

				// Mark as discovered
				discovered[childPageInfo.ID] = true

				// Store in parent's children
				pageInfo.ChildPages = append(pageInfo.ChildPages, childPageInfo)

				// Recursively discover this child page's blocks
				d.discoverChildBlocks(ctx, childPageInfo, opts, currentDepth+1, discovered)
			}
		}

		// Check for pagination
		if !blockList.HasMore || blockList.NextCursor == nil {
			break
		}

		cursor = *blockList.NextCursor
	}
}

// parseSearchResult parses a search result item into PageInfo.
// Returns (pageInfo, isDatabase, error).
// If the item cannot be parsed as a Page, it tries to parse as Database.
func (d *discoverer) parseSearchResult(item interface{}, discovered map[string]bool) (*PageInfo, bool, error) {
	// Try parsing as Page first
	pageData, err := json.Marshal(item)
	if err != nil {
		return nil, false, fmt.Errorf("marshal item: %w", err)
	}

	var page Page
	if err := json.Unmarshal(pageData, &page); err == nil && page.ID != "" {
		// Successfully parsed as Page
		if page.Object == "page" {
			// Skip if already discovered
			if discovered[page.ID] {
				return nil, false, nil
			}

			pageInfo := &PageInfo{
				ID:             page.ID,
				Title:          d.extractPageTitle(&page),
				CreatedTime:    page.CreatedTime,
				LastEditedTime: page.LastEditedTime,
				IsDatabase:     false,
			}
			return pageInfo, false, nil
		}
	}

	// Try parsing as Database
	var db Database
	if err := json.Unmarshal(pageData, &db); err == nil && db.ID != "" {
		if db.Object == "database" {
			// Skip if already discovered
			if discovered[db.ID] {
				return nil, false, nil
			}

			// Extract database title from RichText
			dbTitle := d.extractDatabaseTitle(&db)

			pageInfo := &PageInfo{
				ID:             db.ID,
				Title:          dbTitle,
				CreatedTime:    db.CreatedTime,
				LastEditedTime: db.LastEditedTime,
				IsDatabase:     true,
			}
			return pageInfo, true, nil
		}
	}

	return nil, false, fmt.Errorf("could not parse as Page or Database")
}

// extractPageTitle extracts a human-readable title from a page.
// Returns the URL as fallback if no title can be extracted.
func (d *discoverer) extractPageTitle(page *Page) string {
	// Try to get title from properties (most common case)
	if page.Properties != nil {
		// Look for "title" property
		if titleProp, ok := page.Properties["title"]; ok {
			if titleMap, ok := titleProp.(map[string]interface{}); ok {
				if titleList, ok := titleMap["title"].([]interface{}); ok && len(titleList) > 0 {
					if richText, ok := titleList[0].(map[string]interface{}); ok {
						if plainText, ok := richText["plain_text"].(string); ok && plainText != "" {
							return plainText
						}
					}
				}
			}
		}
	}

	// Fallback to page ID if no title found
	return page.URL
}

// extractDatabaseTitle extracts a human-readable title from a database.
func (d *discoverer) extractDatabaseTitle(db *Database) string {
	if db.Title != nil && len(db.Title) > 0 {
		return db.Title[0].PlainText
	}

	// Fallback to database ID
	return db.URL
}
