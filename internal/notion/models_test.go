package notion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPageBasicUnmarshal tests unmarshaling a basic page from JSON.
func TestPageBasicUnmarshal(t *testing.T) {
	data := loadFixture(t, "page_basic.json")

	var page Page
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatalf("failed to unmarshal page: %v", err)
	}

	// Verify all expected fields are populated
	if page.ID != "123e4567-e89b-12d3-a456-426614174000" {
		t.Errorf("expected ID 123e4567-e89b-12d3-a456-426614174000, got %s", page.ID)
	}
	if page.Object != "page" {
		t.Errorf("expected object 'page', got %s", page.Object)
	}
	if page.Archived != false {
		t.Errorf("expected archived false, got %v", page.Archived)
	}
	if page.InTrash != false {
		t.Errorf("expected in_trash false, got %v", page.InTrash)
	}
	if page.IsLocked != false {
		t.Errorf("expected is_locked false, got %v", page.IsLocked)
	}
	if page.CreatedBy == nil {
		t.Errorf("expected created_by to be populated")
	}
	if page.LastEditedBy == nil {
		t.Errorf("expected last_edited_by to be populated")
	}
	if page.Icon == nil {
		t.Errorf("expected icon to be populated")
	}
	if page.Cover == nil {
		t.Errorf("expected cover to be populated")
	}
	if page.Parent == nil {
		t.Errorf("expected parent to be populated")
	}
	if page.Properties == nil {
		t.Errorf("expected properties to be populated")
	}
	if page.URL == "" {
		t.Errorf("expected URL to be populated")
	}
	if !page.CreatedTime.Equal(time.Date(2024, 2, 17, 10, 30, 0, 0, time.UTC)) {
		t.Errorf("expected created_time to match, got %v", page.CreatedTime)
	}
	if !page.LastEditedTime.Equal(time.Date(2024, 2, 17, 15, 45, 30, 0, time.UTC)) {
		t.Errorf("expected last_edited_time to match, got %v", page.LastEditedTime)
	}
}

// TestParagraphBlockUnmarshal tests unmarshaling a paragraph block.
func TestParagraphBlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_paragraph.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "paragraph" {
		t.Errorf("expected type 'paragraph', got %s", block.Type)
	}
	if block.Paragraph == nil {
		t.Errorf("expected paragraph to be populated")
	}
	if len(block.Paragraph.RichText) == 0 {
		t.Errorf("expected rich_text to be populated")
	}
	if block.Paragraph.RichText[0].Type != "text" {
		t.Errorf("expected text type, got %s", block.Paragraph.RichText[0].Type)
	}
	if block.Paragraph.RichText[1].Annotations.Bold != true {
		t.Errorf("expected bold annotation in second richtext")
	}
}

// TestHeading1BlockUnmarshal tests unmarshaling a heading 1 block.
func TestHeading1BlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_heading1.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "heading_1" {
		t.Errorf("expected type 'heading_1', got %s", block.Type)
	}
	if block.Heading1 == nil {
		t.Errorf("expected heading_1 to be populated")
	}
	if len(block.Heading1.RichText) == 0 {
		t.Errorf("expected rich_text to be populated")
	}
}

// TestCodeBlockUnmarshal tests unmarshaling a code block.
func TestCodeBlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_code.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "code" {
		t.Errorf("expected type 'code', got %s", block.Type)
	}
	if block.Code == nil {
		t.Errorf("expected code to be populated")
	}
	if block.Code.Language == nil || *block.Code.Language != "javascript" {
		t.Errorf("expected language 'javascript'")
	}
}

// TestToDoBlockUnmarshal tests unmarshaling a to-do block.
func TestToDoBlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_todo.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "to_do" {
		t.Errorf("expected type 'to_do', got %s", block.Type)
	}
	if block.ToDo == nil {
		t.Errorf("expected to_do to be populated")
	}
	if block.ToDo.Checked == nil || *block.ToDo.Checked != true {
		t.Errorf("expected checked to be true")
	}
}

// TestImageBlockUnmarshal tests unmarshaling an image block.
func TestImageBlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_image.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "image" {
		t.Errorf("expected type 'image', got %s", block.Type)
	}
	if block.Image == nil {
		t.Errorf("expected image to be populated")
	}
	if block.Image.Type != "external" {
		t.Errorf("expected image type 'external'")
	}
	if block.Image.External == nil || block.Image.External.URL == "" {
		t.Errorf("expected external URL to be populated")
	}
	if len(block.Image.Caption) == 0 {
		t.Errorf("expected caption to be populated")
	}
}

// TestTableBlockUnmarshal tests unmarshaling a table block.
func TestTableBlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_table.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "table" {
		t.Errorf("expected type 'table', got %s", block.Type)
	}
	if block.Table == nil {
		t.Errorf("expected table to be populated")
	}
	if block.Table.TableWidth != 3 {
		t.Errorf("expected table width 3, got %d", block.Table.TableWidth)
	}
	if block.Table.HasColumnHeader != true {
		t.Errorf("expected has_column_header to be true")
	}
}

// TestBulletedListBlockUnmarshal tests unmarshaling a bulleted list block.
func TestBulletedListBlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_bulleted_list.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "bulleted_list_item" {
		t.Errorf("expected type 'bulleted_list_item', got %s", block.Type)
	}
	if block.BulletedList == nil {
		t.Errorf("expected bulleted_list_item to be populated")
	}
}

// TestNumberedListBlockUnmarshal tests unmarshaling a numbered list block.
func TestNumberedListBlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_numbered_list.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "numbered_list_item" {
		t.Errorf("expected type 'numbered_list_item', got %s", block.Type)
	}
	if block.NumberedList == nil {
		t.Errorf("expected numbered_list_item to be populated")
	}
}

// TestBookmarkBlockUnmarshal tests unmarshaling a bookmark block.
func TestBookmarkBlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_bookmark.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "bookmark" {
		t.Errorf("expected type 'bookmark', got %s", block.Type)
	}
	if block.Bookmark == nil {
		t.Errorf("expected bookmark to be populated")
	}
	if block.Bookmark.URL != "https://example.com" {
		t.Errorf("expected URL to be https://example.com, got %s", block.Bookmark.URL)
	}
}

// TestCalloutBlockUnmarshal tests unmarshaling a callout block.
func TestCalloutBlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_callout.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "callout" {
		t.Errorf("expected type 'callout', got %s", block.Type)
	}
	if block.Callout == nil {
		t.Errorf("expected callout to be populated")
	}
	if block.Callout.Icon == nil {
		t.Errorf("expected icon to be populated")
	}
	if block.Callout.Icon.Emoji == nil || *block.Callout.Icon.Emoji != "⚠️" {
		t.Errorf("expected emoji ⚠️")
	}
}

// TestDividerBlockUnmarshal tests unmarshaling a divider block.
func TestDividerBlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_divider.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "divider" {
		t.Errorf("expected type 'divider', got %s", block.Type)
	}
	if block.Divider == nil {
		t.Errorf("expected divider to be populated")
	}
}

// TestChildPageBlockUnmarshal tests unmarshaling a child page block.
func TestChildPageBlockUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_child_page.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.Type != "child_page" {
		t.Errorf("expected type 'child_page', got %s", block.Type)
	}
	if block.ChildPage == nil {
		t.Errorf("expected child_page to be populated")
	}
	if block.ChildPage.Title != "Sub Page" {
		t.Errorf("expected title 'Sub Page'")
	}
}

// TestRichTextWithMentionsUnmarshal tests unmarshaling rich text with mentions.
func TestRichTextWithMentionsUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_richtext_mentions.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	richTexts := block.Paragraph.RichText
	if len(richTexts) < 4 {
		t.Errorf("expected at least 4 rich text elements, got %d", len(richTexts))
	}

	// Check mention type
	mentionFound := false
	for _, rt := range richTexts {
		if rt.Type == "mention" && rt.Mention != nil {
			mentionFound = true
			if rt.Mention.Type != "user" {
				t.Errorf("expected mention type 'user'")
			}
			if rt.Mention.User == nil {
				t.Errorf("expected user to be populated")
			}
			break
		}
	}
	if !mentionFound {
		t.Errorf("expected to find a mention in rich text")
	}

	// Check equation type
	equationFound := false
	for _, rt := range richTexts {
		if rt.Type == "equation" && rt.Equation != nil {
			equationFound = true
			if rt.Equation.Expression != "E = mc^2" {
				t.Errorf("expected equation E = mc^2")
			}
			break
		}
	}
	if !equationFound {
		t.Errorf("expected to find an equation in rich text")
	}
}

// TestListBlocksPaginationUnmarshal tests unmarshaling paginated blocks list.
func TestListBlocksPaginationUnmarshal(t *testing.T) {
	data := loadFixture(t, "list_blocks_pagination.json")

	var resp ListBlocksResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal list response: %v", err)
	}

	if resp.Type != "block" {
		t.Errorf("expected type 'block'")
	}
	if len(resp.Results) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(resp.Results))
	}
	if resp.NextCursor == nil || *resp.NextCursor != "next-cursor-token" {
		t.Errorf("expected next_cursor to be set")
	}
	if resp.HasMore != true {
		t.Errorf("expected has_more to be true")
	}
}

// TestDatabaseBasicUnmarshal tests unmarshaling a basic database.
func TestDatabaseBasicUnmarshal(t *testing.T) {
	data := loadFixture(t, "database_basic.json")

	var db Database
	if err := json.Unmarshal(data, &db); err != nil {
		t.Fatalf("failed to unmarshal database: %v", err)
	}

	if db.ID != "db-123" {
		t.Errorf("expected ID db-123")
	}
	if db.Object != "database" {
		t.Errorf("expected object 'database'")
	}
	if len(db.Title) == 0 {
		t.Errorf("expected title to be populated")
	}
	if db.Icon == nil {
		t.Errorf("expected icon to be populated")
	}
	if db.Icon.Emoji == nil || *db.Icon.Emoji != "📊" {
		t.Errorf("expected emoji 📊")
	}
	if len(db.Properties) == 0 {
		t.Errorf("expected properties to be populated")
	}
	if db.IsInline != true {
		t.Errorf("expected is_inline to be true")
	}
}

// TestQueryDatabaseResponseUnmarshal tests unmarshaling a database query response.
func TestQueryDatabaseResponseUnmarshal(t *testing.T) {
	data := loadFixture(t, "query_database_response.json")

	var resp QueryDatabaseResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal query response: %v", err)
	}

	if resp.Type != "page_or_database" {
		t.Errorf("expected type 'page_or_database'")
	}
	if len(resp.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.DatabaseID != "db-123" {
		t.Errorf("expected database_id db-123")
	}
	if resp.NextCursor != nil {
		t.Errorf("expected next_cursor to be nil")
	}
	if resp.HasMore != false {
		t.Errorf("expected has_more to be false")
	}
}

// TestUserUnmarshal tests unmarshaling user objects.
func TestUserUnmarshal(t *testing.T) {
	data := loadFixture(t, "page_basic.json")

	var page Page
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatalf("failed to unmarshal page: %v", err)
	}

	if page.CreatedBy.Object != "user" {
		t.Errorf("expected user object")
	}
	if page.CreatedBy.Type != "person" {
		t.Errorf("expected user type 'person'")
	}
	if page.CreatedBy.ID != "user-123" {
		t.Errorf("expected user ID user-123")
	}
	if page.CreatedBy.Name == nil || *page.CreatedBy.Name != "John Doe" {
		t.Errorf("expected user name 'John Doe'")
	}
	if page.CreatedBy.Email == nil || *page.CreatedBy.Email != "john@example.com" {
		t.Errorf("expected user email 'john@example.com'")
	}
}

// TestParentTypesUnmarshal tests unmarshaling different parent types.
func TestParentTypesUnmarshal(t *testing.T) {
	tests := []struct {
		name         string
		fixture      string
		expectedType string
	}{
		{"page parent", "block_paragraph.json", "page_id"},
		{"workspace parent", "page_basic.json", "workspace"},
		{"database parent", "query_database_response.json", "database_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := loadFixture(t, tt.fixture)

			var v interface{}
			if tt.expectedType == "database_id" {
				var resp QueryDatabaseResponse
				if err := json.Unmarshal(data, &resp); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				v = resp.Results[0]
			} else if tt.expectedType == "page_id" {
				var block Block
				if err := json.Unmarshal(data, &block); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				v = block
			} else {
				var page Page
				if err := json.Unmarshal(data, &page); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				v = page
			}

			var parent *Parent
			switch val := v.(type) {
			case Block:
				parent = val.Parent
			case Page:
				parent = val.Parent
			}

			if parent == nil {
				t.Fatalf("expected parent to be populated")
			}
			if parent.Type != tt.expectedType {
				t.Errorf("expected parent type '%s', got '%s'", tt.expectedType, parent.Type)
			}
		})
	}
}

// TestFileOrEmojiUnmarshal tests unmarshaling file and emoji types.
func TestFileOrEmojiUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		hasEmoji bool
		hasFile  bool
	}{
		{"emoji icon", "page_basic.json", true, false},
		{"external file cover", "page_basic.json", false, true},
		{"emoji callout icon", "block_callout.json", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := loadFixture(t, tt.fixture)

			var page Page
			if err := json.Unmarshal(data, &page); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if tt.name == "emoji icon" && page.Icon != nil {
				if tt.hasEmoji && page.Icon.Emoji == nil {
					t.Errorf("expected emoji to be populated")
				}
			}
			if tt.name == "external file cover" && page.Cover != nil {
				if tt.hasFile && page.Cover.External == nil {
					t.Errorf("expected external file to be populated")
				}
			}
		})
	}
}

// TestTextAnnotationsUnmarshal tests unmarshaling text annotations.
func TestTextAnnotationsUnmarshal(t *testing.T) {
	data := loadFixture(t, "block_paragraph.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	richTexts := block.Paragraph.RichText
	boldText := richTexts[1]

	if boldText.Annotations.Bold != true {
		t.Errorf("expected bold annotation")
	}
	if boldText.Annotations.Italic != false {
		t.Errorf("expected no italic annotation")
	}
	if boldText.Annotations.Color != "default" {
		t.Errorf("expected default color")
	}
}

// TestNullAndOmitEmptyFields tests handling null and omitempty fields.
func TestNullAndOmitEmptyFields(t *testing.T) {
	data := loadFixture(t, "page_basic.json")

	var page Page
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatalf("failed to unmarshal page: %v", err)
	}

	// PublicURL should be nil
	if page.PublicURL != nil {
		t.Errorf("expected public_url to be nil")
	}

	// Cover and Icon should be populated (not nil)
	if page.Cover == nil {
		t.Errorf("expected cover to be populated")
	}
	if page.Icon == nil {
		t.Errorf("expected icon to be populated")
	}
}

// TestBlockWithChildren tests blocks with children.
func TestBlockWithChildren(t *testing.T) {
	data := loadFixture(t, "block_table.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	if block.HasChildren != true {
		t.Errorf("expected has_children to be true")
	}
}

// TestAllBlockTypesInList tests that list response can unmarshal multiple block types.
func TestAllBlockTypesInList(t *testing.T) {
	data := loadFixture(t, "list_blocks_pagination.json")

	var resp ListBlocksResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal list response: %v", err)
	}

	for i, block := range resp.Results {
		if block.Type != "paragraph" {
			t.Errorf("block %d: expected type 'paragraph', got '%s'", i, block.Type)
		}
		if block.Paragraph == nil {
			t.Errorf("block %d: expected paragraph to be populated", i)
		}
	}
}

// TestTimeParsingRFC3339 tests RFC3339 time parsing.
func TestTimeParsingRFC3339(t *testing.T) {
	data := loadFixture(t, "page_basic.json")

	var page Page
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatalf("failed to unmarshal page: %v", err)
	}

	// Verify times were parsed correctly
	expectedCreated := time.Date(2024, 2, 17, 10, 30, 0, 0, time.UTC)
	expectedEdited := time.Date(2024, 2, 17, 15, 45, 30, 0, time.UTC)

	if !page.CreatedTime.Equal(expectedCreated) {
		t.Errorf("expected created_time %v, got %v", expectedCreated, page.CreatedTime)
	}
	if !page.LastEditedTime.Equal(expectedEdited) {
		t.Errorf("expected last_edited_time %v, got %v", expectedEdited, page.LastEditedTime)
	}
}

// TestSyncedBlockUnmarshal tests unmarshaling a synced block.
func TestSyncedBlockUnmarshal(t *testing.T) {
	// Create a synced block inline for this one
	jsonData := []byte(`{
		"object": "block",
		"id": "block-synced-001",
		"type": "synced_block",
		"synced_block": {
			"synced_from": {
				"block_id": "original-block-id"
			}
		},
		"parent": {"type": "page_id", "page_id": "page-123"},
		"created_time": "2024-02-17T10:30:00.000Z",
		"last_edited_time": "2024-02-17T10:30:00.000Z",
		"created_by": {"object": "user", "id": "user-123", "type": "person"},
		"last_edited_by": {"object": "user", "id": "user-123", "type": "person"},
		"has_children": false,
		"archived": false,
		"in_trash": false,
		"is_locked": false
	}`)

	var block Block
	if err := json.Unmarshal(jsonData, &block); err != nil {
		t.Fatalf("failed to unmarshal synced block: %v", err)
	}

	if block.Type != "synced_block" {
		t.Errorf("expected type 'synced_block'")
	}
	if block.SyncedBlock == nil {
		t.Errorf("expected synced_block to be populated")
	}
	if block.SyncedBlock.SyncedFrom == nil || block.SyncedBlock.SyncedFrom.BlockID != "original-block-id" {
		t.Errorf("expected synced_from to be populated correctly")
	}
}

// TestHeading2BlockUnmarshal tests unmarshaling heading 2 block.
func TestHeading2BlockUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"object": "block",
		"id": "block-h2-001",
		"type": "heading_2",
		"heading_2": {
			"rich_text": [{"type": "text", "text": {"content": "Heading 2"}, "annotations": {}, "plain_text": "Heading 2"}],
			"color": "default",
			"is_toggleable": false
		},
		"parent": {"type": "page_id", "page_id": "page-123"},
		"created_time": "2024-02-17T10:30:00.000Z",
		"last_edited_time": "2024-02-17T10:30:00.000Z",
		"created_by": {"object": "user", "id": "user-123", "type": "person"},
		"last_edited_by": {"object": "user", "id": "user-123", "type": "person"},
		"has_children": false,
		"archived": false,
		"in_trash": false,
		"is_locked": false
	}`)

	var block Block
	if err := json.Unmarshal(jsonData, &block); err != nil {
		t.Fatalf("failed to unmarshal heading 2 block: %v", err)
	}

	if block.Type != "heading_2" {
		t.Errorf("expected type 'heading_2'")
	}
	if block.Heading2 == nil {
		t.Errorf("expected heading_2 to be populated")
	}
}

// TestHeading3BlockUnmarshal tests unmarshaling heading 3 block.
func TestHeading3BlockUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"object": "block",
		"id": "block-h3-001",
		"type": "heading_3",
		"heading_3": {
			"rich_text": [{"type": "text", "text": {"content": "Heading 3"}, "annotations": {}, "plain_text": "Heading 3"}],
			"color": "default",
			"is_toggleable": false
		},
		"parent": {"type": "page_id", "page_id": "page-123"},
		"created_time": "2024-02-17T10:30:00.000Z",
		"last_edited_time": "2024-02-17T10:30:00.000Z",
		"created_by": {"object": "user", "id": "user-123", "type": "person"},
		"last_edited_by": {"object": "user", "id": "user-123", "type": "person"},
		"has_children": false,
		"archived": false,
		"in_trash": false,
		"is_locked": false
	}`)

	var block Block
	if err := json.Unmarshal(jsonData, &block); err != nil {
		t.Fatalf("failed to unmarshal heading 3 block: %v", err)
	}

	if block.Type != "heading_3" {
		t.Errorf("expected type 'heading_3'")
	}
	if block.Heading3 == nil {
		t.Errorf("expected heading_3 to be populated")
	}
}

// TestQuoteBlockUnmarshal tests unmarshaling a quote block.
func TestQuoteBlockUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"object": "block",
		"id": "block-quote-001",
		"type": "quote",
		"quote": {
			"rich_text": [{"type": "text", "text": {"content": "To be or not to be"}, "annotations": {}, "plain_text": "To be or not to be"}],
			"color": "default"
		},
		"parent": {"type": "page_id", "page_id": "page-123"},
		"created_time": "2024-02-17T10:30:00.000Z",
		"last_edited_time": "2024-02-17T10:30:00.000Z",
		"created_by": {"object": "user", "id": "user-123", "type": "person"},
		"last_edited_by": {"object": "user", "id": "user-123", "type": "person"},
		"has_children": false,
		"archived": false,
		"in_trash": false,
		"is_locked": false
	}`)

	var block Block
	if err := json.Unmarshal(jsonData, &block); err != nil {
		t.Fatalf("failed to unmarshal quote block: %v", err)
	}

	if block.Type != "quote" {
		t.Errorf("expected type 'quote'")
	}
	if block.Quote == nil {
		t.Errorf("expected quote to be populated")
	}
}

// TestToggleBlockUnmarshal tests unmarshaling a toggle block.
func TestToggleBlockUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"object": "block",
		"id": "block-toggle-001",
		"type": "toggle",
		"toggle": {
			"rich_text": [{"type": "text", "text": {"content": "Click to expand"}, "annotations": {}, "plain_text": "Click to expand"}],
			"color": "default"
		},
		"parent": {"type": "page_id", "page_id": "page-123"},
		"created_time": "2024-02-17T10:30:00.000Z",
		"last_edited_time": "2024-02-17T10:30:00.000Z",
		"created_by": {"object": "user", "id": "user-123", "type": "person"},
		"last_edited_by": {"object": "user", "id": "user-123", "type": "person"},
		"has_children": false,
		"archived": false,
		"in_trash": false,
		"is_locked": false
	}`)

	var block Block
	if err := json.Unmarshal(jsonData, &block); err != nil {
		t.Fatalf("failed to unmarshal toggle block: %v", err)
	}

	if block.Type != "toggle" {
		t.Errorf("expected type 'toggle'")
	}
	if block.Toggle == nil {
		t.Errorf("expected toggle to be populated")
	}
}

// TestVideoBlockUnmarshal tests unmarshaling a video block.
func TestVideoBlockUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"object": "block",
		"id": "block-video-001",
		"type": "video",
		"video": {
			"type": "external",
			"external": {
				"url": "https://example.com/video.mp4"
			},
			"caption": []
		},
		"parent": {"type": "page_id", "page_id": "page-123"},
		"created_time": "2024-02-17T10:30:00.000Z",
		"last_edited_time": "2024-02-17T10:30:00.000Z",
		"created_by": {"object": "user", "id": "user-123", "type": "person"},
		"last_edited_by": {"object": "user", "id": "user-123", "type": "person"},
		"has_children": false,
		"archived": false,
		"in_trash": false,
		"is_locked": false
	}`)

	var block Block
	if err := json.Unmarshal(jsonData, &block); err != nil {
		t.Fatalf("failed to unmarshal video block: %v", err)
	}

	if block.Type != "video" {
		t.Errorf("expected type 'video'")
	}
	if block.Video == nil {
		t.Errorf("expected video to be populated")
	}
	if block.Video.External == nil || block.Video.External.URL != "https://example.com/video.mp4" {
		t.Errorf("expected external video URL")
	}
}

// TestFileBlockUnmarshal tests unmarshaling a file block.
func TestFileBlockUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"object": "block",
		"id": "block-file-001",
		"type": "file",
		"file": {
			"type": "file",
			"file": {
				"url": "https://s3.example.com/file.pdf"
			},
			"caption": []
		},
		"parent": {"type": "page_id", "page_id": "page-123"},
		"created_time": "2024-02-17T10:30:00.000Z",
		"last_edited_time": "2024-02-17T10:30:00.000Z",
		"created_by": {"object": "user", "id": "user-123", "type": "person"},
		"last_edited_by": {"object": "user", "id": "user-123", "type": "person"},
		"has_children": false,
		"archived": false,
		"in_trash": false,
		"is_locked": false
	}`)

	var block Block
	if err := json.Unmarshal(jsonData, &block); err != nil {
		t.Fatalf("failed to unmarshal file block: %v", err)
	}

	if block.Type != "file" {
		t.Errorf("expected type 'file'")
	}
	if block.File == nil {
		t.Errorf("expected file to be populated")
	}
}

// TestEmbedBlockUnmarshal tests unmarshaling an embed block.
func TestEmbedBlockUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"object": "block",
		"id": "block-embed-001",
		"type": "embed",
		"embed": {
			"url": "https://youtube.com/embed/xyz",
			"caption": []
		},
		"parent": {"type": "page_id", "page_id": "page-123"},
		"created_time": "2024-02-17T10:30:00.000Z",
		"last_edited_time": "2024-02-17T10:30:00.000Z",
		"created_by": {"object": "user", "id": "user-123", "type": "person"},
		"last_edited_by": {"object": "user", "id": "user-123", "type": "person"},
		"has_children": false,
		"archived": false,
		"in_trash": false,
		"is_locked": false
	}`)

	var block Block
	if err := json.Unmarshal(jsonData, &block); err != nil {
		t.Fatalf("failed to unmarshal embed block: %v", err)
	}

	if block.Type != "embed" {
		t.Errorf("expected type 'embed'")
	}
	if block.Embed == nil {
		t.Errorf("expected embed to be populated")
	}
	if block.Embed.URL != "https://youtube.com/embed/xyz" {
		t.Errorf("expected embed URL")
	}
}

// TestChildDatabaseBlockUnmarshal tests unmarshaling a child database block.
func TestChildDatabaseBlockUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"object": "block",
		"id": "block-child-db-001",
		"type": "child_database",
		"child_database": {
			"title": "Nested Database"
		},
		"parent": {"type": "page_id", "page_id": "page-123"},
		"created_time": "2024-02-17T10:30:00.000Z",
		"last_edited_time": "2024-02-17T10:30:00.000Z",
		"created_by": {"object": "user", "id": "user-123", "type": "person"},
		"last_edited_by": {"object": "user", "id": "user-123", "type": "person"},
		"has_children": false,
		"archived": false,
		"in_trash": false,
		"is_locked": false
	}`)

	var block Block
	if err := json.Unmarshal(jsonData, &block); err != nil {
		t.Fatalf("failed to unmarshal child database block: %v", err)
	}

	if block.Type != "child_database" {
		t.Errorf("expected type 'child_database'")
	}
	if block.ChildDatabase == nil {
		t.Errorf("expected child_database to be populated")
	}
	if block.ChildDatabase.Title != "Nested Database" {
		t.Errorf("expected child database title")
	}
}

// loadFixture is a helper function to load JSON test fixtures.
func loadFixture(t *testing.T, filename string) []byte {
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", filename, err)
	}
	return data
}

// TestErrorUnmarshal tests unmarshaling an error response.
func TestErrorUnmarshal(t *testing.T) {
	jsonData := []byte(`{
		"object": "error",
		"status": 401,
		"code": "unauthorized",
		"message": "Invalid Bearer token"
	}`)

	var err Error
	if err2 := json.Unmarshal(jsonData, &err); err2 != nil {
		t.Fatalf("failed to unmarshal error: %v", err2)
	}

	if err.Object != "error" {
		t.Errorf("expected object 'error'")
	}
	if err.Status != 401 {
		t.Errorf("expected status 401")
	}
	if err.Code != "unauthorized" {
		t.Errorf("expected code 'unauthorized'")
	}
	if err.Message != "Invalid Bearer token" {
		t.Errorf("expected error message")
	}
}

// TestBlockMetadataFields tests all block metadata fields.
func TestBlockMetadataFields(t *testing.T) {
	data := loadFixture(t, "block_paragraph.json")

	var block Block
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("failed to unmarshal block: %v", err)
	}

	// Test all metadata fields
	if block.ID == "" {
		t.Errorf("expected block ID")
	}
	if block.Object != "block" {
		t.Errorf("expected object 'block'")
	}
	if block.Parent == nil {
		t.Errorf("expected parent")
	}
	if block.CreatedTime.IsZero() {
		t.Errorf("expected created_time")
	}
	if block.LastEditedTime.IsZero() {
		t.Errorf("expected last_edited_time")
	}
	if block.CreatedBy == nil {
		t.Errorf("expected created_by")
	}
	if block.LastEditedBy == nil {
		t.Errorf("expected last_edited_by")
	}
	if block.Archived || block.InTrash || block.IsLocked {
		t.Errorf("expected archive fields to be false")
	}
}

// TestPageMetadataFields tests all page metadata fields.
func TestPageMetadataFields(t *testing.T) {
	data := loadFixture(t, "page_basic.json")

	var page Page
	if err := json.Unmarshal(data, &page); err != nil {
		t.Fatalf("failed to unmarshal page: %v", err)
	}

	// Test all metadata fields
	if page.ID == "" {
		t.Errorf("expected page ID")
	}
	if page.Object != "page" {
		t.Errorf("expected object 'page'")
	}
	if page.Parent == nil {
		t.Errorf("expected parent")
	}
	if page.CreatedTime.IsZero() {
		t.Errorf("expected created_time")
	}
	if page.LastEditedTime.IsZero() {
		t.Errorf("expected last_edited_time")
	}
	if page.CreatedBy == nil {
		t.Errorf("expected created_by")
	}
	if page.LastEditedBy == nil {
		t.Errorf("expected last_edited_by")
	}
	if page.Archived || page.InTrash || page.IsLocked {
		t.Errorf("expected archive fields to be false")
	}
}
