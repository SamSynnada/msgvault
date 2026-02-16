# Component 1: Notion Data Models - Detailed Plan

**Component**: `internal/notion/models.go`
**Who**: Agent: ModelBuilder
**Dependencies**: None (standalone types)
**Duration**: 2-3 hours
**Testing**: 100% unit tests with fixtures

---

## 📋 Responsibilities

Define Go types for all Notion API v2025-09-03 responses with proper JSON unmarshaling.

## 🎯 Requirements

### Input (from real API)
- Notion Page response
- Notion Block response
- Query database response
- Search response
- Rich text objects
- User objects

### Output
Clean, strongly-typed Go structs that unmarshal JSON correctly.

## 📐 Type Definitions to Implement

```go
package notion

import "time"

// Page represents a Notion page object
type Page struct {
    ID             string                    `json:"id"`
    Object         string                    `json:"object"`
    CreatedTime    time.Time                 `json:"created_time"`
    LastEditedTime time.Time                 `json:"last_edited_time"`
    CreatedBy      *User                     `json:"created_by"`
    LastEditedBy   *User                     `json:"last_edited_by"`
    Cover          *FileOrEmoji              `json:"cover"`
    Icon           *FileOrEmoji              `json:"icon"`
    Parent         *Parent                   `json:"parent"`
    Archived       bool                      `json:"archived"`
    InTrash        bool                      `json:"in_trash"`
    IsLocked       bool                      `json:"is_locked"`
    Properties     map[string]interface{}    `json:"properties"`
    URL            string                    `json:"url"`
    PublicURL      *string                   `json:"public_url"`
}

// Block represents a Notion block (page content)
type Block struct {
    ID             string                    `json:"id"`
    Object         string                    `json:"object"`
    Parent         *BlockParent              `json:"parent"`
    CreatedTime    time.Time                 `json:"created_time"`
    LastEditedTime time.Time                 `json:"last_edited_time"`
    CreatedBy      *User                     `json:"created_by"`
    LastEditedBy   *User                     `json:"last_edited_by"`
    HasChildren    bool                      `json:"has_children"`
    Archived       bool                      `json:"archived"`
    InTrash        bool                      `json:"in_trash"`
    IsLocked       bool                      `json:"is_locked"`

    // Type-specific content (one will be populated)
    Type           string                    `json:"type"`
    Paragraph      *ParagraphBlock           `json:"paragraph,omitempty"`
    Heading1       *HeadingBlock             `json:"heading_1,omitempty"`
    Heading2       *HeadingBlock             `json:"heading_2,omitempty"`
    Heading3       *HeadingBlock             `json:"heading_3,omitempty"`
    BulletedList   *ListBlock                `json:"bulleted_list_item,omitempty"`
    NumberedList   *ListBlock                `json:"numbered_list_item,omitempty"`
    ToDo           *ToDoBlock                `json:"to_do,omitempty"`
    Code           *CodeBlock                `json:"code,omitempty"`
    Quote          *RichTextBlock            `json:"quote,omitempty"`
    Callout        *CalloutBlock             `json:"callout,omitempty"`
    Divider        interface{}               `json:"divider,omitempty"`
    Image          *MediaBlock               `json:"image,omitempty"`
    Video          *MediaBlock               `json:"video,omitempty"`
    File           *FileBlock                `json:"file,omitempty"`
    ChildDatabase  *ChildDatabaseBlock       `json:"child_database,omitempty"`
    ChildPage      *ChildPageBlock           `json:"child_page,omitempty"`
    SyncedBlock    *SyncedBlockBlock         `json:"synced_block,omitempty"`
    Table          *TableBlock               `json:"table,omitempty"`
    Toggle         *ToggleBlock              `json:"toggle,omitempty"`
    Bookmark       *BookmarkBlock            `json:"bookmark,omitempty"`
    Embed          *EmbedBlock               `json:"embed,omitempty"`
}

// RichText represents formatted text with annotations
type RichText struct {
    Type        string                `json:"type"` // "text", "mention", "equation"
    Text        *TextContent          `json:"text,omitempty"`
    Mention     *Mention              `json:"mention,omitempty"`
    Equation    *Equation             `json:"equation,omitempty"`
    Annotations *TextAnnotations      `json:"annotations"`
    PlainText   string                `json:"plain_text"`
    Href        *string               `json:"href"`
}

// Parent represents the parent of a page or block
type Parent struct {
    Type             string `json:"type"` // "workspace", "page_id", "block_id", "data_source"
    Workspace        bool   `json:"workspace,omitempty"`
    PageID           string `json:"page_id,omitempty"`
    BlockID          string `json:"block_id,omitempty"`
    DataSourceID     string `json:"data_source_id,omitempty"`
}

// User represents a Notion user
type User struct {
    Object string `json:"object"`
    ID     string `json:"id"`
}

// ... (other type definitions)
```

## 🧪 Test Cases

### Fixture Files (save real API responses)
- `testdata/page_simple.json` - Basic page
- `testdata/page_with_cover.json` - Page with cover image
- `testdata/block_paragraph.json` - Paragraph block
- `testdata/block_heading.json` - Heading blocks
- `testdata/block_list.json` - Bullet/numbered list
- `testdata/block_code.json` - Code block
- `testdata/block_child_database.json` - Database block
- `testdata/block_child_page.json` - Nested page
- `testdata/search_response.json` - Search results

### Unit Tests
```go
func TestPageUnmarshal_Simple(t *testing.T) {
    // Load testdata/page_simple.json
    // Unmarshal to Page
    // Assert all fields populated
}

func TestBlockUnmarshal_AllTypes(t *testing.T) {
    // For each fixture file
    // Unmarshal and verify Type field matches
}

func TestRichTextFormatting(t *testing.T) {
    // Unmarshal rich text with bold, italic, code
    // Verify annotations parsed
}

func TestTimeHandling(t *testing.T) {
    // Verify created_time, last_edited_time parse correctly
}
```

## 📋 Deliverables

1. `internal/notion/models.go` - All type definitions
2. `internal/notion/models_test.go` - Unit tests with fixtures
3. `internal/notion/testdata/` - JSON fixtures from real API
4. Test coverage: **100%** (all types and marshaling paths)

## ✅ Definition of Done

- [ ] All types defined matching v2025-09-03 schema
- [ ] JSON tags correct for all fields
- [ ] Unit tests use real API fixtures
- [ ] Unmarshaling of all block types verified
- [ ] Time parsing works correctly
- [ ] Null/empty values handled properly
- [ ] Code passes `go vet` and `gofmt`
- [ ] No external dependencies added

---

# Component 2: Rate Limiter - Detailed Plan

**Component**: `internal/notion/ratelimit.go`
**Who**: Agent: ClientBuilder (part 1)
**Dependencies**: `time`, `sync`
**Duration**: 1-2 hours

---

## 📋 Responsibilities

Enforce Notion API rate limit: 3 requests/second average, with burst capacity.

## 🎯 Requirements

### Input
- Operation type
- Context (for cancellation)

### Output
- Block until rate limit allows
- Or return error if context cancelled

## 🔧 Implementation (Token Bucket Algorithm)

```go
type Operation string

const (
    OpSearch        Operation = "search"
    OpGetPage       Operation = "get_page"
    OpGetBlocks     Operation = "get_blocks"
    OpQueryDatabase Operation = "query_db"
    OpUpdatePage    Operation = "update_page"
)

// RateLimiter enforces Notion API rate limits (3 req/sec)
type RateLimiter struct {
    // Token bucket fields
    mu           sync.Mutex
    tokens       float64
    lastRefill   time.Time
    tokensPerSec float64
    maxBurst     float64

    // Metrics for testing/debugging
    acquiredCount map[Operation]int64
}

func NewRateLimiter(tokensPerSec float64) *RateLimiter {
    return &RateLimiter{
        tokens:       0,
        lastRefill:   time.Now(),
        tokensPerSec: tokensPerSec,
        maxBurst:     10.0,  // Allow 10-token burst
        acquiredCount: make(map[Operation]int64),
    }
}

// Acquire waits until a token is available or context is cancelled
func (rl *RateLimiter) Acquire(ctx context.Context, op Operation) error {
    // Token bucket logic
    // Wait if no tokens available
    // Return error on context cancellation
}

// Stats returns rate limiter metrics
func (rl *RateLimiter) Stats() map[Operation]int64 {
    // Return operation counts for testing
}
```

## 🧪 Test Cases

```go
func TestRateLimiter_AllowsInitialRequests(t *testing.T) {
    // First 3 requests should not block
}

func TestRateLimiter_BlocksExcess(t *testing.T) {
    // 4th request should block
}

func TestRateLimiter_Context Cancellation(t *testing.T) {
    // Acquire returns error if context cancelled while waiting
}

func TestRateLimiter_Concurrent(t *testing.T) {
    // Multiple goroutines respecting rate limit
    // Verify total throughput ≈ 3 req/sec
}

func TestRateLimiter_Burst(t *testing.T) {
    // After waiting, allows burst of up to maxBurst requests
}
```

## 📋 Deliverables

1. `internal/notion/ratelimit.go` - Token bucket implementation
2. `internal/notion/ratelimit_test.go` - Concurrent stress tests
3. Test coverage: **100%**

---

# Component 3: Notion API Client - Detailed Plan

**Component**: `internal/notion/client.go`
**Who**: Agent: ClientBuilder (part 2)
**Dependencies**: Models, RateLimiter
**Duration**: 4-6 hours

---

## 📋 Responsibilities

Low-level HTTP communication with Notion API, with retry logic and error handling.

## 🎯 Interface Contract

```go
// NotionAPI defines all Notion API operations
type NotionAPI interface {
    SearchPages(ctx context.Context, opts *SearchOpts) (*SearchResult, error)
    GetPage(ctx context.Context, pageID string) (*Page, error)
    GetDatabaseMetadata(ctx context.Context, dbID string) (*Database, error)
    GetBlockChildren(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error)
    QueryDatabase(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error)
    Close() error
}

// Client implements NotionAPI
type Client struct {
    httpClient  *http.Client
    token       string
    apiVersion  string // "2025-09-03"
    rateLimiter *RateLimiter
    logger      *slog.Logger
}
```

## 🏗️ Key Methods

### SearchPages
```go
type SearchOpts struct {
    Filter   *SearchFilter
    Sort     *SearchSort
    PageSize int // 1-100
}

type SearchFilter struct {
    Property string // "object"
    Value    string // "page", "database"
}

func (c *Client) SearchPages(ctx context.Context, opts *SearchOpts) (*SearchResult, error) {
    // POST /v1/search
    // Retry on transient errors (429, 500-503)
    // Return parsed SearchResult
}
```

### GetBlockChildren
```go
type BlockListOpts struct {
    PageSize   int    // 1-100
    StartCursor string // For pagination
}

func (c *Client) GetBlockChildren(ctx context.Context, pageID string, opts *BlockListOpts) (*BlockList, error) {
    // GET /v1/blocks/{page_id}/children
    // Handle pagination
    // Return blocks
}
```

### QueryDatabase
```go
type QueryOpts struct {
    Filter     *DatabaseFilter
    Sorts      []*DatabaseSort
    PageSize   int
    StartCursor string
}

func (c *Client) QueryDatabase(ctx context.Context, dbID string, opts *QueryOpts) (*QueryResult, error) {
    // POST /v1/databases/{db_id}/query
    // Support timestamp filtering for incremental sync
}
```

## 🧪 Test Strategy

### Unit Tests (All with HTTP Mock Server)
```go
func TestClient_SearchPages_Success(t *testing.T) {
    // httptest server returns valid response
    // Verify client parses and returns result
}

func TestClient_Retry_On429(t *testing.T) {
    // Server returns 429 once, then 200
    // Verify client retries automatically
}

func TestClient_Backoff(t *testing.T) {
    // Verify exponential backoff timing
    // E.g., 429 response: wait 1s, then 2s, then 4s...
}

func TestClient_ContextCancellation(t *testing.T) {
    // Context cancelled mid-request
    // Returns error without hanging
}

func TestClient_RateLiming(t *testing.T) {
    // Multiple requests respect rate limit
    // Verify rate limiter is called
}
```

### Integration Tests (Real Credentials)
```go
func TestClient_SearchPages_Real(t *testing.T) {
    if testing.Short() {
        t.Skip("integration test")
    }
    // Real API, verify search returns pages
}
```

## 🔄 Error Handling

Notion API errors:
```
400 Bad Request
401 Unauthorized
403 Forbidden (page not shared)
404 Not Found
429 Too Many Requests (rate limit)
500-503 Server errors
```

All should be wrapped with context:
```go
if resp.StatusCode == http.StatusTooManyRequests {
    return fmt.Errorf("rate limited: %w", parseNotionError(resp))
}
```

## 📋 Deliverables

1. `internal/notion/client.go` - HTTP client with retry logic
2. `internal/notion/client_test.go` - Comprehensive unit tests
3. Mocks for mock.go if needed
4. Test coverage: **95%+**

---

# Component 4: Markdown Converter - Detailed Plan

**Component**: `internal/notion/markdown.go`
**Who**: Agent: ConverterBuilder
**Dependencies**: Models
**Duration**: 6-8 hours

---

## 📋 Responsibilities

Transform Notion blocks (DOM-like structure) into readable markdown format.

## 🎯 Core Algorithm

```
For each block:
  1. Convert based on block.Type
  2. Extract rich text with formatting
  3. Handle indentation/nesting
  4. Preserve links, code, formatting
  5. Concat with newlines
```

## 🔄 Block Type Handlers

### Priority 0 (MVP)
```
paragraph → plain text
heading_1/2/3 → # ## ###
bulleted_list_item → - item
numbered_list_item → 1. item (renumber sequentially)
to_do → - [ ] or - [x]
```

### Priority 1 (Phase 2)
```
code → ```lang\ncode\n```
quote → > quote
callout → > emoji Text
child_page → ## Nested Page Title
child_database → (reference or section marker)
```

### Priority 2 (Phase 3)
```
image → ![alt](url)
file → [filename](url)
divider → ---
table → (HTML table or CSV)
toggle → ▶ Title + indented content
```

## 📐 Core Types

```go
type MarkdownConverter interface {
    ConvertToMarkdown(blocks []*Block) (string, error)
    ConvertBlockToMarkdown(block *Block) (string, error)
}

type converter struct {
    logger *slog.Logger
}

// NewConverter creates a markdown converter
func NewConverter(logger *slog.Logger) MarkdownConverter {
    return &converter{logger: logger}
}
```

## 📄 Output Format (Example)

Input: Notion page with heading, paragraph, list
```
heading_1: "My Title"
paragraph: "This is a paragraph with **bold** text"
bulleted_list_item: "First item"
bulleted_list_item: "Second item"
```

Output:
```
# My Title

This is a paragraph with **bold** text

- First item
- Second item
```

## 🧪 Test Strategy

### Fixture-Based Tests
```go
func TestConvertBlock_Paragraph(t *testing.T) {
    block := &Block{
        Type: "paragraph",
        Paragraph: &ParagraphBlock{
            RichText: []RichText{
                {
                    Type: "text",
                    Text: &TextContent{Content: "Hello world"},
                    Annotations: &TextAnnotations{Bold: true},
                },
            },
        },
    }

    got, err := converter.ConvertBlockToMarkdown(block)
    want := "**Hello world**"
    // Assert
}

func TestConvertBlocks_List(t *testing.T) {
    blocks := []*Block{
        {Type: "bulleted_list_item", ...},
        {Type: "bulleted_list_item", ...},
    }

    got, err := converter.ConvertToMarkdown(blocks)
    // Assert contains "- item1\n- item2"
}

func TestConvertBlocks_Mixed(t *testing.T) {
    // heading + paragraph + list
    // Verify output is properly formatted markdown
}
```

### Rich Text Formatting
```go
func TestConvertRichText_Bold(t *testing.T) {
    rt := &RichText{
        Text: &TextContent{Content: "bold"},
        Annotations: &TextAnnotations{Bold: true},
    }
    got := convertRichText(rt)
    assert.Equal(t, "**bold**", got)
}

func TestConvertRichText_Link(t *testing.T) {
    rt := &RichText{
        Text: &TextContent{
            Content: "link",
            Link: &LinkInfo{URL: "https://example.com"},
        },
    }
    got := convertRichText(rt)
    assert.Equal(t, "[link](https://example.com)", got)
}

func TestConvertRichText_CombinedFormatting(t *testing.T) {
    // bold + italic + code + link
    // Ensure proper nesting: ***content***
}
```

### Nested Content
```go
func TestConvertBlocks_NestedLists(t *testing.T) {
    // Bullet list with children
    // Verify indentation
    // Input: block.HasChildren=true
    // Output: "- item\n  - nested"
}
```

## 📋 Deliverables

1. `internal/notion/markdown.go` - Converter implementation
2. `internal/notion/markdown_test.go` - Comprehensive tests
3. Test fixtures covering all block types
4. Test coverage: **90%+**

---

Continue with remaining components...
