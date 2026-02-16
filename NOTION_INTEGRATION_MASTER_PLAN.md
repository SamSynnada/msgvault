# Notion Integration Master Plan

**Status**: Planning Phase
**Last Updated**: 2026-02-16
**Author**: Claude Code

---

## 📊 Overview

This document outlines the complete architecture for integrating Notion as a data source into msgvault, alongside the existing Gmail integration. The plan follows Test-Driven Development (TDD) principles with unit, integration, and E2E tests at each stage.

**Key Goals:**
- Enable users to archive Notion pages and databases alongside Gmail
- Maintain msgvault's FTS5 search across Notion content
- Support incremental sync using `last_edited_time` filtering
- Follow existing code patterns and styling conventions
- Enable parallel development via sub-agents

---

## 🏗️ Architecture Overview

### Design Principles

1. **Platform Abstraction**: Treat Notion as a platform alongside Gmail
2. **Separation of Concerns**: Each component has a single responsibility
3. **Testability**: Mock all external APIs, test in layers
4. **Consistency**: Follow msgvault's established patterns

### High-Level Flow

```
User runs: ./msgvault add-account notion --token <secret>
                ↓
[1] NotionOAuth Manager stores token
                ↓
User runs: ./msgvault sync-full notion --workspace myworkspace
                ↓
[2] Discovery Service finds all pages/databases
                ↓
[3] Content Fetcher retrieves pages and blocks
                ↓
[4] Markdown Converter transforms blocks → markdown
                ↓
[5] Store Extensions persist to SQLite
                ↓
User runs: ./msgvault tui
                ↓
[6] Query Engine searches across Gmail + Notion
```

---

## 📦 Sub-Components (Parallel Development)

### Component 1: Notion API Client (`internal/notion/client.go`)

**Responsibility**: Low-level HTTP communication with Notion API

**Interfaces**:
```go
// NotionAPI defines the Notion API interface for mocking
type NotionAPI interface {
    SearchPages(ctx context.Context, opts *SearchOpts) (*SearchResult, error)
    GetPage(ctx context.Context, pageID string) (*Page, error)
    GetBlocks(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error)
    QueryDatabase(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error)
    GetDatabase(ctx context.Context, dbID string) (*Database, error)
}

// Client implements NotionAPI
type Client struct {
    httpClient   *http.Client
    token        string
    rateLimiter  *RateLimiter
    logger       *slog.Logger
}
```

**Key Features**:
- Bearer token authentication
- Rate limiting (3 req/sec)
- Exponential backoff on 429 errors
- Request context propagation
- Retry logic for transient failures

**Dependencies**:
- `http` stdlib
- `slog` for logging
- Internal rate limiter

**Test Strategy**:
- **Unit Tests**: Mock HTTP server, test request/response parsing
- **Integration Tests**: Real API with test credentials (narrow scope)
- **E2E**: Full workspace search and block retrieval

---

### Component 2: Notion Data Models (`internal/notion/models.go`)

**Responsibility**: Type definitions for Notion API responses

**Structures**:
```go
type Page struct {
    ID            string
    CreatedTime   time.Time
    LastEditedTime time.Time
    LastEditedBy  *User
    Title         string
    CreatedBy     *User
    Properties    map[string]interface{}
    Parent        *Parent
    IsLocked      bool
    InTrash       bool
    Archived      bool
    URL           string
}

type Block struct {
    ID            string
    Type          string  // "paragraph", "heading_1", "child_database", etc.
    CreatedTime   time.Time
    LastEditedTime time.Time
    LastEditedBy  *User
    HasChildren   bool
    IsLocked      bool
    InTrash       bool
    Content       map[string]interface{}  // Type-specific content
}

type RichText struct {
    Type        string
    Text        *TextObj
    Annotations *Annotations
    PlainText   string
    Href        *string
}

type QueryResult struct {
    Results    []*Page
    HasMore    bool
    NextCursor string
}
```

**Key Considerations**:
- Handle v2025-09-03 API response structure
- Support new fields: `last_edited_by`, `is_locked`, `in_trash`
- Multi-source database support (parent types)
- JSON marshaling/unmarshaling

**Test Strategy**:
- **Unit Tests**: JSON deserialization with real API payloads
- **Fixtures**: Save real API responses as JSON files
- **Validation**: Ensure all required fields are populated

---

### Component 3: Notion OAuth Manager (`internal/notion/oauth.go`)

**Responsibility**: Token acquisition and storage for Notion integration

**Interfaces**:
```go
type NotionOAuthManager interface {
    GetTokenSource(ctx context.Context, workspace string) (string, error)
    StoreToken(workspace string, token string) error
    LoadToken(workspace string) (string, error)
    RevokeToken(workspace string) error
}
```

**Key Features**:
- Store tokens in `~/.msgvault/tokens/notion_{workspace_id}.json`
- Support internal integration tokens (simple setup)
- Support OAuth for future multi-workspace usage
- Token validation on load

**Test Strategy**:
- **Unit Tests**: Temp directory for token storage/retrieval
- **Validation Tests**: Malformed tokens rejected
- **E2E**: Real token with test workspace

---

### Component 4: Notion Discovery & Traversal (`internal/notion/discovery.go`)

**Responsibility**: Find all pages and databases in a Notion workspace

**Interfaces**:
```go
type PageDiscoverer interface {
    DiscoverPages(ctx context.Context, opts *DiscoveryOpts) (<-chan *PageInfo, error)
}

type PageInfo struct {
    ID            string
    Title         string
    CreatedTime   time.Time
    LastEditedTime time.Time
    IsDatabase    bool
    ChildDatabases []*DatabaseInfo
    ChildPages    []*PageInfo
}

type DatabaseInfo struct {
    ID    string
    Title string
}
```

**Key Features**:
- Pagination through search results
- Recursive traversal of child pages
- Detect child databases in blocks
- Handle multi-source databases (new in 2025-09-03)
- Streaming results for large workspaces

**Test Strategy**:
- **Unit Tests**: Mock page hierarchy, pagination
- **Integration Tests**: Real workspace discovery (narrow scope)
- **Edge Cases**: Empty pages, deeply nested pages, circular refs (if possible)

---

### Component 5: Block Content Fetcher (`internal/notion/content_fetcher.go`)

**Responsibility**: Retrieve and organize all blocks for a page

**Interfaces**:
```go
type ContentFetcher interface {
    FetchPageContent(ctx context.Context, pageID string) (*PageContent, error)
    FetchDatabaseRows(ctx context.Context, dbID string, opts *QueryOpts) ([]*DatabaseRow, error)
}

type PageContent struct {
    PageID   string
    Blocks   []*Block
    HasMore  bool
    NextCursor string
}

type DatabaseRow struct {
    ID         string
    Title      string
    Properties map[string]interface{}
    URL        string
}
```

**Key Features**:
- Pagination for blocks (100 per request)
- Recursive block fetching (handle child blocks)
- Database query with timestamp filtering
- Property extraction for databases
- Concurrent fetching (with rate limit respecting)

**Test Strategy**:
- **Unit Tests**: Mock blocks at different depths
- **Integration Tests**: Real page content fetching
- **Stress**: Large pages (1000+ blocks)

---

### Component 6: Markdown Converter (`internal/notion/markdown.go`)

**Responsibility**: Convert Notion blocks to markdown format

**Interfaces**:
```go
type MarkdownConverter interface {
    ConvertToMarkdown(blocks []*Block) (string, error)
    ConvertBlockToMarkdown(block *Block) (string, error)
}
```

**Block Type Mappings**:

| Notion Block Type | Markdown Output | Priority |
|-------------------|-----------------|----------|
| `paragraph` | Plain text | P0 |
| `heading_1`, `heading_2`, `heading_3` | `#`, `##`, `###` | P0 |
| `bulleted_list_item` | `- item` | P0 |
| `numbered_list_item` | `1. item` | P0 |
| `to_do` | `- [ ] task` or `- [x] done` | P0 |
| `code` | ` ```lang\ncode\n``` ` | P1 |
| `quote` | `> quote` | P1 |
| `callout` | `> [emoji] callout text` | P1 |
| `divider` | `---` | P2 |
| `table` | HTML table | P2 |
| `image` | `![alt](url)` | P2 |
| `file` | `[filename](url)` | P2 |
| `child_page` | `## Nested Page Title` | P1 |
| `child_database` | Database section reference | P1 |

**Key Features**:
- Rich text formatting (bold, italic, code, links)
- Nested list handling (proper indentation)
- URL extraction and preservation
- Color/annotation handling (simplify to basic formatting)
- Metadata preservation in YAML frontmatter

**Test Strategy**:
- **Unit Tests**: Each block type individually
- **Fixtures**: Real Notion blocks converted to markdown
- **Validation**: Compare with hand-crafted markdown
- **Edge Cases**: Empty blocks, mixed formatting, deeply nested lists

---

### Component 7: Store Extensions (`internal/store/notion_schema.sql` + Go code)

**Responsibility**: Extend SQLite schema to store Notion content

**New Tables**:
```sql
-- Notion workspaces/accounts
CREATE TABLE IF NOT EXISTS notion_sources (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    workspace_name TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    last_sync_cursor TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Notion page to message mapping
CREATE TABLE IF NOT EXISTS notion_pages (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL,
    title TEXT NOT NULL,
    created_time DATETIME NOT NULL,
    last_edited_time DATETIME NOT NULL,
    created_by_id TEXT,
    last_edited_by_id TEXT,
    is_locked BOOLEAN,
    in_trash BOOLEAN,
    parent_type TEXT,  -- 'workspace', 'page', 'database', 'data_source'
    parent_id TEXT,
    FOREIGN KEY(source_id) REFERENCES notion_sources(id)
);

-- Link notion_pages to messages table
CREATE TABLE IF NOT EXISTS notion_message_map (
    message_id INTEGER PRIMARY KEY,
    notion_page_id TEXT NOT NULL,
    FOREIGN KEY(message_id) REFERENCES messages(id),
    FOREIGN KEY(notion_page_id) REFERENCES notion_pages(id)
);
```

**Key Features**:
- Normalize Notion data into existing message schema
- Map Notion pages to messages (reuse FTS5)
- Store sync state for incremental updates
- Preserve Notion metadata

**Store Interface Extensions**:
```go
type Store interface {
    // Existing methods...

    // Notion-specific
    CreateNotionSource(ctx context.Context, src *NotionSource) error
    SaveNotionPage(ctx context.Context, page *NotionPage) error
    UpdateSyncCursor(ctx context.Context, sourceID, cursor string) error
    ListNotionSources(ctx context.Context) ([]*NotionSource, error)
}
```

**Test Strategy**:
- **Unit Tests**: Schema validation, constraint checking
- **Integration Tests**: Data persistence verification
- **Transaction Tests**: Rollback on error

---

### Component 8: Notion Adapter (`internal/notion/adapter.go`)

**Responsibility**: High-level orchestration (implements Syncer interface)

**Interfaces**:
```go
// NotionSyncer orchestrates the full sync workflow
type NotionSyncer interface {
    SyncFull(ctx context.Context, opts *SyncOptions) (*SyncResult, error)
    SyncIncremental(ctx context.Context, opts *SyncOptions) (*SyncResult, error)
}

type SyncOptions struct {
    WorkspaceID    string
    StartCursor    string  // For incremental
    MaxPages       int     // Limit for testing
    BatchSize      int
    CheckpointInterval int
}

type SyncResult struct {
    PagesProcessed     int
    PagesStored        int
    BlocksProcessed    int
    AttachmentsStored  int
    Errors             []string
    Duration           time.Duration
    NextCursor         string
}
```

**Workflow**:
1. Load token from storage
2. Create API client
3. Discover pages/databases
4. For each page:
   - Fetch blocks
   - Convert to markdown
   - Store in database
   - Handle child databases
5. Save sync state
6. Report results

**Test Strategy**:
- **Unit Tests**: Orchestration logic with mocks
- **Integration Tests**: Full workflow with real API
- **E2E Tests**: Real workspace, verify stored data

---

### Component 9: Rate Limiter (`internal/notion/ratelimit.go`)

**Responsibility**: Enforce Notion API rate limits (3 req/sec)

**Design**:
- Token bucket algorithm
- Shared across all Notion clients
- Configurable burst capacity
- Integration with context cancellation

**Test Strategy**:
- **Unit Tests**: Token bucket math
- **Concurrency Tests**: Multiple goroutines respecting limits
- **Stress**: Burst and sustained load patterns

---

### Component 10: Integration Tests (`internal/notion/integration_test.go`)

**Responsibility**: E2E testing with real API/test workspace

**Test Coverage**:
- Authentication (valid/invalid tokens)
- Page discovery (pagination, child pages)
- Block retrieval (all block types)
- Markdown conversion accuracy
- Database queries with filtering
- Incremental sync (changed pages detected)
- Error handling (permission denied, rate limits)

**Test Data**:
- Real credentials in `.env`
- Test pages covering all block types
- Test database with rows
- Verification queries to ensure data persisted

---

## 🔄 Component Dependency Graph

```
NotionOAuthManager
    ↓
NotionClient ← NotionAPI interface
    ├─ RateLimiter
    └─ Models (JSON parsing)

NotionClient ↓
Discovery (PageDiscoverer interface)
    ↓
ContentFetcher (ContentFetcher interface)
    ├─ BlockFetcher
    └─ DatabaseQueryer

ContentFetcher ↓
MarkdownConverter (MarkdownConverter interface)
    ├─ RichTextFormatter
    └─ Block type handlers

MarkdownConverter ↓
NotionAdapter (NotionSyncer interface)
    ├─ Discovery
    ├─ ContentFetcher
    ├─ MarkdownConverter
    ├─ StoreExtensions
    └─ RateLimiter

NotionAdapter ↓
CLI Command (cmd/notion_sync.go)
```

---

## 📋 Code Style & Conventions

**From existing msgvault codebase:**

### Package Structure
```
internal/notion/
├── client.go           # API client
├── client_test.go
├── models.go           # Type definitions
├── models_test.go
├── oauth.go            # Token management
├── oauth_test.go
├── discovery.go        # Page discovery
├── discovery_test.go
├── content_fetcher.go  # Block retrieval
├── content_fetcher_test.go
├── markdown.go         # Block → markdown
├── markdown_test.go
├── adapter.go          # High-level orchestration
├── adapter_test.go
├── ratelimit.go        # Rate limiting
├── ratelimit_test.go
├── integration_test.go # E2E tests
├── mock.go             # Test mocks
├── mock_test.go
└── testutil.go         # Test helpers
```

### Naming Conventions
- `NewXxx()` for constructors
- `WithXxx()` for options
- `Interface` suffix for contracts
- `*_test.go` for tests in same package
- `*_mock.go` for mocks

### Error Handling
```go
// Wrap errors with context
return fmt.Errorf("fetch blocks for page %s: %w", pageID, err)

// Define domain-specific errors
var ErrPageNotFound = errors.New("page not found")
var ErrRateLimited = errors.New("rate limit exceeded")
```

### Logging
```go
// Use structured logging
logger := slog.Default()
logger.InfoContext(ctx, "sync started",
    "workspace", workspaceID,
    "page_count", pageCount,
)
logger.WarnContext(ctx, "rate limited", "retry_after", retryAfter)
```

### Testing Patterns
```go
// Table-driven tests
func TestConvertBlock(t *testing.T) {
    tests := []struct {
        name    string
        block   *Block
        want    string
        wantErr bool
    }{
        {"paragraph", &Block{...}, "hello world", false},
        {"heading", &Block{...}, "# Title", false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ConvertBlockToMarkdown(tt.block)
            if (err != nil) != tt.wantErr {
                t.Errorf("got error %v, want %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got %q, want %q", got, tt.want)
            }
        })
    }
}

// Mock objects for dependencies
type mockNotionAPI struct {
    searchPagesFn func(ctx context.Context, opts *SearchOpts) (*SearchResult, error)
}

// Builder pattern for test data
page := NewPageBuilder().
    WithID("abc123").
    WithTitle("My Page").
    Build()
```

### Options Pattern
```go
type ClientOption func(*Client)

func WithRateLimiter(rl *RateLimiter) ClientOption {
    return func(c *Client) {
        c.rateLimiter = rl
    }
}

func NewClient(token string, opts ...ClientOption) *Client {
    c := &Client{token: token}
    for _, opt := range opts {
        opt(c)
    }
    return c
}
```

---

## 🧪 Testing Strategy (TDD)

### Test Pyramid

```
             E2E Tests
            /    |    \
        /    Integration  \
    /        Tests       \
  Unit Tests (Core Logic)
```

#### Layer 1: Unit Tests (Per Component)
- **Scope**: Single function/method
- **Mocks**: All external dependencies
- **Speed**: < 100ms per test
- **Examples**:
  - ConvertBlockToMarkdown with various block types
  - RateLimiter token distribution
  - JSON deserialization of Page responses

#### Layer 2: Integration Tests (Component Interactions)
- **Scope**: 2-3 components working together
- **Mocks**: Notion API (httptest), Store (in-memory)
- **Speed**: < 1s per test
- **Examples**:
  - Discovery → ContentFetcher → MarkdownConverter pipeline
  - Adapter orchestrating full sync with mocks
  - Store persisting converted pages

#### Layer 3: E2E Tests (Real API)
- **Scope**: Full workflow with test workspace
- **Mocks**: None (real API)
- **Speed**: 5-30s per test (acceptable for E2E)
- **Examples**:
  - discover-pages (scope: 1 workspace)
  - fetch-page-content (scope: 1 page)
  - sync-full (scope: 5-10 test pages)

### Test Data Strategy

**Stage 1: Unit (Mocked)**
```go
// JSON fixtures from real API
var fixturePageJSON = `{
    "object": "page",
    "id": "abc123...",
    ...
}`

func TestUnmarshalPage(t *testing.T) {
    var page Page
    json.Unmarshal([]byte(fixturePageJSON), &page)
    // Assert fields populated correctly
}
```

**Stage 2: Integration (In-Memory Mocks)**
```go
type mockAPI struct {
    pages map[string]*Page
}

func (m *mockAPI) GetPage(ctx context.Context, pageID string) (*Page, error) {
    if p, ok := m.pages[pageID]; ok {
        return p, nil
    }
    return nil, ErrPageNotFound
}

// Use in adapter tests
adapter := NewAdapter(mockAPI{pages: testPages}, mockStore)
```

**Stage 3: E2E (Real API)**
```go
func TestDiscoverPages_Real(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping E2E test")
    }

    token := os.Getenv("NOTION_INTEGRATION_TOKEN")
    if token == "" {
        t.Skip("NOTION_INTEGRATION_TOKEN not set")
    }

    client := NewClient(token)
    pages, err := client.SearchPages(context.Background(), ...)
    // Assert discovery works
}
```

---

## ⚙️ Execution Order & Dependencies

### Recommended Implementation Sequence

**Phase 1: Foundation (Testable Independently)**
1. ✓ **Models** (`internal/notion/models.go`)
   - No dependencies
   - Can validate from real API payloads
   - All tests use fixtures

2. ✓ **Rate Limiter** (`internal/notion/ratelimit.go`)
   - No dependencies
   - Pure algorithm testing
   - Concurrent stress testing

3. **OAuth Manager** (`internal/notion/oauth.go`)
   - Minimal dependency: file I/O
   - Mock file system
   - Real token validation in E2E

**Phase 2: Core API Integration (Per-Component Tests)**
4. **Notion Client** (`internal/notion/client.go`)
   - Depends on: Models, RateLimiter
   - HTTP mock server for tests
   - Real API in E2E

5. **Markdown Converter** (`internal/notion/markdown.go`)
   - Depends on: Models
   - Table-driven tests with fixtures
   - No external I/O

6. **Discovery Service** (`internal/notion/discovery.go`)
   - Depends on: Client, Models
   - Mock client for pagination tests
   - Real API in E2E

7. **Content Fetcher** (`internal/notion/content_fetcher.go`)
   - Depends on: Client, Models
   - Mock client for block hierarchies
   - Real API in E2E

**Phase 3: Integration (Multiple Components)**
8. **Store Extensions** (`internal/store/` updates)
   - Depends on: Store package
   - In-memory SQLite for tests
   - Real persistence verification

9. **Notion Adapter** (`internal/notion/adapter.go`)
   - Depends on: All above components
   - Mock all dependencies
   - Full workflow with mocks

**Phase 4: E2E & CLI**
10. **Integration Tests** (`internal/notion/integration_test.go`)
    - Real API with test workspace
    - Narrow scope verification
    - Data integrity checks

11. **CLI Command** (`cmd/notion_sync.go`)
    - Depends on: Adapter, OAuth Manager
    - Integration with existing CLI

---

## 📈 Quality Gates

### Before Committing Each Component:
- [ ] `go fmt ./...` passes
- [ ] `go vet ./...` passes
- [ ] All new tests pass (`go test ./...`)
- [ ] No unit test flakes (run 3× locally)
- [ ] Coverage > 80% for new code
- [ ] Mocks are realistic (match real API behavior)

### Before Merging Phase:
- [ ] All integration tests pass
- [ ] Error scenarios handled (400, 429, 500 responses)
- [ ] Rate limit behavior verified
- [ ] Pagination tested with large result sets

### Before Release:
- [ ] E2E tests pass with real API
- [ ] Real workspace sync verified
- [ ] CLI commands work end-to-end
- [ ] Performance benchmarked (sync time, storage size)

---

## 🔧 Configuration & TODOs

### Environment Setup (.env)
```
NOTION_INTEGRATION_TOKEN=ntn_...
NOTION_WORKSPACE_ID=workspace123
NOTION_TEST_PAGE_ID=page_abc123...
NOTION_TEST_DATABASE_ID=db_def456...
```

### TODOs for Implementation

- [ ] Prepare test fixtures (save real API responses as JSON)
- [ ] Define NotionAPI interface contract
- [ ] Create mock implementations for all interfaces
- [ ] Set up in-memory SQLite for store tests
- [ ] Prepare test workspace (diverse page types)
- [ ] Define performance benchmarks
- [ ] Implement backward-compatibility testing
- [ ] Plan webhook support (post-MVP)

---

## 📞 Sub-Agent Assignment

Each sub-component below can be assigned to a dedicated sub-agent:

1. **Agent: ModelBuilder** → Component 2 (Models)
2. **Agent: ClientBuilder** → Components 1, 9 (Client + RateLimiter)
3. **Agent: ConverterBuilder** → Component 6 (Markdown Converter)
4. **Agent: OAuthBuilder** → Component 3 (OAuth Manager)
5. **Agent: DiscoveryBuilder** → Component 4 (Discovery)
6. **Agent: FetcherBuilder** → Component 5 (Content Fetcher)
7. **Agent: AdapterBuilder** → Component 8 (Adapter)
8. **Agent: StoreBuilder** → Component 7 (Store Extensions)
9. **Agent: IntegrationBuilder** → Components 10 (Integration Tests)

Each agent:
- Works on their component independently
- Writes isolated unit tests from the start
- Follows the TDD approach outlined
- Documents integration points with other components
- Provides a summary of assumptions and limitations

---

## ✅ Definition of Done

A component is "done" when:
1. All unit tests pass (100% of happy path + error cases)
2. Mocks are realistic and match real API behavior
3. No integration with other components required (can test in isolation)
4. Code follows msgvault conventions
5. Error handling is comprehensive
6. Logging is structured and helpful
7. Documentation includes usage examples
8. README for the component added to `experiments/notion/`

---

## 🎯 Success Metrics

- ✅ All 10 components complete and passing tests
- ✅ Zero conflicts between component implementations
- ✅ E2E sync of 5-10 test pages succeeds
- ✅ Markdown quality matches manual export
- ✅ Incremental sync detects real page changes
- ✅ Read-only operations (no data loss risk)
- ✅ Comparable performance to Gmail sync

---

**Next Step**: Sub-agents review this plan, clarify assumptions, and provide feedback before commencing implementation.
