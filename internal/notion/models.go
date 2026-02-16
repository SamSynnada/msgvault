// Package notion provides type definitions for the Notion API v2025-09-03.
package notion

import "time"

// Page represents a Notion page object.
type Page struct {
	ID              string                 `json:"id"`
	Object          string                 `json:"object"`
	CreatedTime     time.Time              `json:"created_time"`
	LastEditedTime  time.Time              `json:"last_edited_time"`
	CreatedBy       *User                  `json:"created_by"`
	LastEditedBy    *User                  `json:"last_edited_by"`
	Cover           *FileOrEmoji           `json:"cover"`
	Icon            *FileOrEmoji           `json:"icon"`
	Parent          *Parent                `json:"parent"`
	Archived        bool                   `json:"archived"`
	InTrash         bool                   `json:"in_trash"`
	IsLocked        bool                   `json:"is_locked"`
	Properties      map[string]interface{} `json:"properties"`
	URL             string                 `json:"url"`
	PublicURL       *string                `json:"public_url"`
}

// Block represents a Notion block object.
type Block struct {
	ID              string                 `json:"id"`
	Object          string                 `json:"object"`
	Parent          *Parent                `json:"parent"`
	CreatedTime     time.Time              `json:"created_time"`
	LastEditedTime  time.Time              `json:"last_edited_time"`
	CreatedBy       *User                  `json:"created_by"`
	LastEditedBy    *User                  `json:"last_edited_by"`
	HasChildren     bool                   `json:"has_children"`
	Type            string                 `json:"type"`
	Archived        bool                   `json:"archived"`
	InTrash         bool                   `json:"in_trash"`
	IsLocked        bool                   `json:"is_locked"`

	// Block type-specific content
	Paragraph       *ParagraphBlock       `json:"paragraph,omitempty"`
	Heading1        *Heading1Block        `json:"heading_1,omitempty"`
	Heading2        *Heading2Block        `json:"heading_2,omitempty"`
	Heading3        *Heading3Block        `json:"heading_3,omitempty"`
	BulletedList    *BulletedListBlock    `json:"bulleted_list_item,omitempty"`
	NumberedList    *NumberedListBlock    `json:"numbered_list_item,omitempty"`
	ToDo            *ToDoBlock            `json:"to_do,omitempty"`
	Code            *CodeBlock            `json:"code,omitempty"`
	Quote           *QuoteBlock           `json:"quote,omitempty"`
	Callout         *CalloutBlock         `json:"callout,omitempty"`
	Divider         *DividerBlock         `json:"divider,omitempty"`
	Image           *ImageBlock           `json:"image,omitempty"`
	Video           *VideoBlock           `json:"video,omitempty"`
	File            *FileBlock            `json:"file,omitempty"`
	ChildDatabase   *ChildDatabaseBlock   `json:"child_database,omitempty"`
	ChildPage       *ChildPageBlock       `json:"child_page,omitempty"`
	SyncedBlock      *SyncedBlockBlock     `json:"synced_block,omitempty"`
	Table           *TableBlock           `json:"table,omitempty"`
	Toggle          *ToggleBlock          `json:"toggle,omitempty"`
	Bookmark        *BookmarkBlock        `json:"bookmark,omitempty"`
	Embed           *EmbedBlock           `json:"embed,omitempty"`
}

// RichText represents a rich text object with formatting.
type RichText struct {
	Type        string           `json:"type"`
	Text        *TextContent     `json:"text,omitempty"`
	Mention     *Mention         `json:"mention,omitempty"`
	Equation    *EquationContent `json:"equation,omitempty"`
	Annotations *TextAnnotations `json:"annotations,omitempty"`
	PlainText   string           `json:"plain_text"`
	Href        *string          `json:"href"`
}

// TextContent represents the text part of rich text.
type TextContent struct {
	Content string  `json:"content"`
	Link    *string `json:"link"`
}

// Mention represents a mention in rich text.
type Mention struct {
	Type        string       `json:"type"`
	User        *User        `json:"user,omitempty"`
	Page        *PageRef     `json:"page,omitempty"`
	Database    *DatabaseRef `json:"database,omitempty"`
	Date        *DateRange   `json:"date,omitempty"`
	LinkPreview *LinkPreview `json:"link_preview,omitempty"`
	TemplateMention *TemplateMention `json:"template_mention,omitempty"`
}

// PageRef represents a reference to a page.
type PageRef struct {
	ID string `json:"id"`
}

// DatabaseRef represents a reference to a database.
type DatabaseRef struct {
	ID string `json:"id"`
}

// DateRange represents a date range in mentions.
type DateRange struct {
	Start string  `json:"start"`
	End   *string `json:"end"`
}

// LinkPreview represents a link preview in mentions.
type LinkPreview struct {
	URL string `json:"url"`
}

// TemplateMention represents a template mention placeholder.
type TemplateMention struct {
	Type string `json:"type"`
}

// EquationContent represents mathematical equation content.
type EquationContent struct {
	Expression string `json:"expression"`
}

// TextAnnotations represents formatting applied to text.
type TextAnnotations struct {
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
	Color         string `json:"color"`
}

// Parent represents the parent of a page or block.
type Parent struct {
	Type         string  `json:"type"`
	PageID       *string `json:"page_id,omitempty"`
	BlockID      *string `json:"block_id,omitempty"`
	DatabaseID  *string `json:"database_id,omitempty"`
	WorkspaceID *string `json:"workspace,omitempty"`
}

// User represents a Notion user.
type User struct {
	Object string  `json:"object"`
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Name   *string `json:"name,omitempty"`
	Email  *string `json:"email,omitempty"`
}

// FileOrEmoji represents either a file object or an emoji.
type FileOrEmoji struct {
	Type   string      `json:"type"`
	File   *FileObject `json:"file,omitempty"`
	Emoji  *string     `json:"emoji,omitempty"`
	External *ExternalFile `json:"external,omitempty"`
}

// FileObject represents a file stored in Notion.
type FileObject struct {
	URL        string `json:"url"`
	ExpiryTime time.Time `json:"expiry_time,omitempty"`
}

// ExternalFile represents an externally hosted file.
type ExternalFile struct {
	URL string `json:"url"`
}

// ParagraphBlock represents a paragraph block.
type ParagraphBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color"`
}

// Heading1Block represents a heading 1 block.
type Heading1Block struct {
	RichText   []RichText `json:"rich_text"`
	Color      string     `json:"color"`
	IsTogglable bool       `json:"is_toggleable"`
}

// Heading2Block represents a heading 2 block.
type Heading2Block struct {
	RichText   []RichText `json:"rich_text"`
	Color      string     `json:"color"`
	IsTogglable bool       `json:"is_toggleable"`
}

// Heading3Block represents a heading 3 block.
type Heading3Block struct {
	RichText   []RichText `json:"rich_text"`
	Color      string     `json:"color"`
	IsTogglable bool       `json:"is_toggleable"`
}

// BulletedListBlock represents a bulleted list item block.
type BulletedListBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color"`
}

// NumberedListBlock represents a numbered list item block.
type NumberedListBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color"`
}

// ToDoBlock represents a to-do list item block.
type ToDoBlock struct {
	RichText []RichText `json:"rich_text"`
	Checked  *bool      `json:"checked"`
	Color    string     `json:"color"`
}

// CodeBlock represents a code block.
type CodeBlock struct {
	RichText []RichText `json:"rich_text"`
	Language *string    `json:"language"`
	Caption  []RichText `json:"caption"`
}

// QuoteBlock represents a quote block.
type QuoteBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color"`
}

// CalloutBlock represents a callout block.
type CalloutBlock struct {
	RichText []RichText   `json:"rich_text"`
	Icon     *FileOrEmoji `json:"icon"`
	Color    string       `json:"color"`
}

// DividerBlock represents a divider block.
type DividerBlock struct {
	// No additional properties
}

// ImageBlock represents an image block.
type ImageBlock struct {
	Type     string       `json:"type"`
	File     *FileObject  `json:"file,omitempty"`
	External *ExternalFile `json:"external,omitempty"`
	Caption  []RichText   `json:"caption"`
}

// VideoBlock represents a video block.
type VideoBlock struct {
	Type     string       `json:"type"`
	File     *FileObject  `json:"file,omitempty"`
	External *ExternalFile `json:"external,omitempty"`
	Caption  []RichText   `json:"caption"`
}

// FileBlock represents a file block.
type FileBlock struct {
	Type     string       `json:"type"`
	File     *FileObject  `json:"file,omitempty"`
	External *ExternalFile `json:"external,omitempty"`
	Caption  []RichText   `json:"caption"`
}

// ChildDatabaseBlock represents a child database block.
type ChildDatabaseBlock struct {
	Title string `json:"title"`
}

// ChildPageBlock represents a child page block.
type ChildPageBlock struct {
	Title string `json:"title"`
}

// SyncedBlockBlock represents a synced block.
type SyncedBlockBlock struct {
	SyncedFrom *SyncedFrom `json:"synced_from,omitempty"`
	Children   []string    `json:"children,omitempty"`
}

// SyncedFrom represents the source of a synced block.
type SyncedFrom struct {
	BlockID string `json:"block_id"`
}

// TableBlock represents a table block.
type TableBlock struct {
	TableWidth      int  `json:"table_width"`
	HasColumnHeader bool `json:"has_column_header"`
	HasRowHeader    bool `json:"has_row_header"`
}

// ToggleBlock represents a toggle block.
type ToggleBlock struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color"`
}

// BookmarkBlock represents a bookmark block.
type BookmarkBlock struct {
	URL     string     `json:"url"`
	Caption []RichText `json:"caption"`
}

// EmbedBlock represents an embed block.
type EmbedBlock struct {
	URL     string `json:"url"`
	Caption []RichText `json:"caption"`
}

// Database represents a Notion database object.
type Database struct {
	ID              string                 `json:"id"`
	Object          string                 `json:"object"`
	CreatedTime     time.Time              `json:"created_time"`
	LastEditedTime  time.Time              `json:"last_edited_time"`
	CreatedBy       *User                  `json:"created_by"`
	LastEditedBy    *User                  `json:"last_edited_by"`
	Title           []RichText             `json:"title"`
	Description     []RichText             `json:"description"`
	Icon            *FileOrEmoji           `json:"icon"`
	Cover           *FileOrEmoji           `json:"cover"`
	Properties      map[string]interface{} `json:"properties"`
	Parent          *Parent                `json:"parent"`
	URL             string                 `json:"url"`
	PublicURL       *string                `json:"public_url"`
	IsInline        bool                   `json:"is_inline"`
	Archived        bool                   `json:"archived"`
	InTrash         bool                   `json:"in_trash"`
	IsLocked        bool                   `json:"is_locked"`
}

// QueryDatabaseResponse represents the response from a database query.
type QueryDatabaseResponse struct {
	Results            []Page `json:"results"`
	NextCursor         *string `json:"next_cursor"`
	HasMore            bool    `json:"has_more"`
	Type               string  `json:"type"`
	DatabaseID         string  `json:"database_id"`
}

// ListBlocksResponse represents the response from listing blocks.
type ListBlocksResponse struct {
	Results    []Block `json:"results"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	Type       string  `json:"type"`
	BlockID    string  `json:"block_id"`
}

// ListPagesResponse represents the response from listing pages.
type ListPagesResponse struct {
	Results    []Page  `json:"results"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
	Type       string  `json:"type"`
}

// Error represents a Notion API error response.
type Error struct {
	Object  string `json:"object"`
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
