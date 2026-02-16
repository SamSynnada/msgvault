package notion

import (
	"log/slog"
	"os"
	"testing"
)

func TestNewConverter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	if converter == nil {
		t.Fatal("NewConverter returned nil")
	}

	// Test with nil logger (should use default)
	converter2 := NewConverter(nil)
	if converter2 == nil {
		t.Fatal("NewConverter with nil logger returned nil")
	}
}

// Table-driven tests for paragraph blocks
func TestConvertParagraph(t *testing.T) {
	tests := []struct {
		name     string
		block    *Block
		expected string
	}{
		{
			name: "empty paragraph",
			block: &Block{
				Type: "paragraph",
				Paragraph: &ParagraphBlock{
					RichText: []RichText{},
				},
			},
			expected: "",
		},
		{
			name: "simple paragraph",
			block: &Block{
				Type: "paragraph",
				Paragraph: &ParagraphBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Hello world",
							Text: &TextContent{
								Content: "Hello world",
							},
						},
					},
				},
			},
			expected: "Hello world",
		},
		{
			name: "paragraph with nil",
			block: &Block{
				Type:      "paragraph",
				Paragraph: nil,
			},
			expected: "",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertBlockToMarkdown(tt.block)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Table-driven tests for heading blocks
func TestConvertHeadings(t *testing.T) {
	tests := []struct {
		name     string
		block    *Block
		expected string
	}{
		{
			name: "heading 1",
			block: &Block{
				Type: "heading_1",
				Heading1: &Heading1Block{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Title",
							Text: &TextContent{
								Content: "Title",
							},
						},
					},
					IsTogglable: false,
				},
			},
			expected: "# Title",
		},
		{
			name: "heading 2",
			block: &Block{
				Type: "heading_2",
				Heading2: &Heading2Block{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Subtitle",
							Text: &TextContent{
								Content: "Subtitle",
							},
						},
					},
					IsTogglable: false,
				},
			},
			expected: "## Subtitle",
		},
		{
			name: "heading 3",
			block: &Block{
				Type: "heading_3",
				Heading3: &Heading3Block{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Subheading",
							Text: &TextContent{
								Content: "Subheading",
							},
						},
					},
					IsTogglable: false,
				},
			},
			expected: "### Subheading",
		},
		{
			name: "empty heading 1",
			block: &Block{
				Type: "heading_1",
				Heading1: &Heading1Block{
					RichText: []RichText{},
				},
			},
			expected: "# ",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertBlockToMarkdown(tt.block)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Table-driven tests for list blocks
func TestConvertBulletedList(t *testing.T) {
	tests := []struct {
		name     string
		block    *Block
		expected string
	}{
		{
			name: "simple bulleted list",
			block: &Block{
				Type: "bulleted_list_item",
				BulletedList: &BulletedListBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Item one",
							Text: &TextContent{
								Content: "Item one",
							},
						},
					},
				},
			},
			expected: "- Item one",
		},
		{
			name: "empty bulleted list",
			block: &Block{
				Type: "bulleted_list_item",
				BulletedList: &BulletedListBlock{
					RichText: []RichText{},
				},
			},
			expected: "- ",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertBlockToMarkdown(tt.block)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Table-driven tests for numbered lists
func TestConvertNumberedList(t *testing.T) {
	tests := []struct {
		name     string
		blocks   []*Block
		expected string
	}{
		{
			name: "sequential numbered list",
			blocks: []*Block{
				{
					Type: "numbered_list_item",
					NumberedList: &NumberedListBlock{
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "First",
								Text: &TextContent{
									Content: "First",
								},
							},
						},
					},
				},
				{
					Type: "numbered_list_item",
					NumberedList: &NumberedListBlock{
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "Second",
								Text: &TextContent{
									Content: "Second",
								},
							},
						},
					},
				},
				{
					Type: "numbered_list_item",
					NumberedList: &NumberedListBlock{
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "Third",
								Text: &TextContent{
									Content: "Third",
								},
							},
						},
					},
				},
			},
			expected: "1. First\n2. Second\n3. Third",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToMarkdown(tt.blocks)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Table-driven tests for to-do blocks
func TestConvertToDo(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name     string
		block    *Block
		expected string
	}{
		{
			name: "unchecked todo",
			block: &Block{
				Type: "to_do",
				ToDo: &ToDoBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Buy milk",
							Text: &TextContent{
								Content: "Buy milk",
							},
						},
					},
					Checked: &falseVal,
				},
			},
			expected: "- [ ] Buy milk",
		},
		{
			name: "checked todo",
			block: &Block{
				Type: "to_do",
				ToDo: &ToDoBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Buy milk",
							Text: &TextContent{
								Content: "Buy milk",
							},
						},
					},
					Checked: &trueVal,
				},
			},
			expected: "- [x] Buy milk",
		},
		{
			name: "todo with nil checked",
			block: &Block{
				Type: "to_do",
				ToDo: &ToDoBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Task",
							Text: &TextContent{
								Content: "Task",
							},
						},
					},
					Checked: nil,
				},
			},
			expected: "- [ ] Task",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertBlockToMarkdown(tt.block)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Table-driven tests for code blocks
func TestConvertCode(t *testing.T) {
	lang := "go"
	tests := []struct {
		name     string
		block    *Block
		expected string
	}{
		{
			name: "code block with language",
			block: &Block{
				Type: "code",
				Code: &CodeBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "func main() {}",
							Text: &TextContent{
								Content: "func main() {}",
							},
						},
					},
					Language: &lang,
					Caption:  []RichText{},
				},
			},
			expected: "```go\nfunc main() {}\n```",
		},
		{
			name: "code block without language",
			block: &Block{
				Type: "code",
				Code: &CodeBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "plain code",
							Text: &TextContent{
								Content: "plain code",
							},
						},
					},
					Language: nil,
					Caption:  []RichText{},
				},
			},
			expected: "```\nplain code\n```",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertBlockToMarkdown(tt.block)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Table-driven tests for quote blocks
func TestConvertQuote(t *testing.T) {
	tests := []struct {
		name     string
		block    *Block
		expected string
	}{
		{
			name: "simple quote",
			block: &Block{
				Type: "quote",
				Quote: &QuoteBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Be yourself",
							Text: &TextContent{
								Content: "Be yourself",
							},
						},
					},
				},
			},
			expected: "> Be yourself",
		},
		{
			name: "empty quote",
			block: &Block{
				Type: "quote",
				Quote: &QuoteBlock{
					RichText: []RichText{},
				},
			},
			expected: "> ",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertBlockToMarkdown(tt.block)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Table-driven tests for callout blocks
func TestConvertCallout(t *testing.T) {
	emoji := "💡"
	tests := []struct {
		name     string
		block    *Block
		expected string
	}{
		{
			name: "callout with emoji",
			block: &Block{
				Type: "callout",
				Callout: &CalloutBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Important",
							Text: &TextContent{
								Content: "Important",
							},
						},
					},
					Icon: &FileOrEmoji{
						Type:  "emoji",
						Emoji: &emoji,
					},
				},
			},
			expected: "> 💡 Important",
		},
		{
			name: "callout without emoji",
			block: &Block{
				Type: "callout",
				Callout: &CalloutBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Note this",
							Text: &TextContent{
								Content: "Note this",
							},
						},
					},
					Icon: nil,
				},
			},
			expected: "> 📌 Note this",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertBlockToMarkdown(tt.block)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Table-driven tests for divider blocks
func TestConvertDivider(t *testing.T) {
	tests := []struct {
		name     string
		block    *Block
		expected string
	}{
		{
			name: "divider block",
			block: &Block{
				Type:    "divider",
				Divider: &DividerBlock{},
			},
			expected: "---",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertBlockToMarkdown(tt.block)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Table-driven tests for child blocks
func TestConvertChildBlocks(t *testing.T) {
	tests := []struct {
		name     string
		block    *Block
		expected string
	}{
		{
			name: "child page",
			block: &Block{
				Type: "child_page",
				ChildPage: &ChildPageBlock{
					Title: "Nested Page",
				},
			},
			expected: "## Nested Page",
		},
		{
			name: "child page empty title",
			block: &Block{
				Type: "child_page",
				ChildPage: &ChildPageBlock{
					Title: "",
				},
			},
			expected: "## Untitled",
		},
		{
			name: "child database",
			block: &Block{
				Type: "child_database",
				ChildDatabase: &ChildDatabaseBlock{
					Title: "My Database",
				},
			},
			expected: "## My Database (database)",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertBlockToMarkdown(tt.block)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Tests for rich text formatting
func TestRichTextFormatting(t *testing.T) {
	tests := []struct {
		name     string
		block    *Block
		expected string
	}{
		{
			name: "bold text",
			block: &Block{
				Type: "paragraph",
				Paragraph: &ParagraphBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "bold text",
							Text: &TextContent{
								Content: "bold text",
							},
							Annotations: &TextAnnotations{
								Bold: true,
							},
						},
					},
				},
			},
			expected: "**bold text**",
		},
		{
			name: "italic text",
			block: &Block{
				Type: "paragraph",
				Paragraph: &ParagraphBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "italic text",
							Text: &TextContent{
								Content: "italic text",
							},
							Annotations: &TextAnnotations{
								Italic: true,
							},
						},
					},
				},
			},
			expected: "*italic text*",
		},
		{
			name: "code text",
			block: &Block{
				Type: "paragraph",
				Paragraph: &ParagraphBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "code",
							Text: &TextContent{
								Content: "code",
							},
							Annotations: &TextAnnotations{
								Code: true,
							},
						},
					},
				},
			},
			expected: "`code`",
		},
		{
			name: "strikethrough text",
			block: &Block{
				Type: "paragraph",
				Paragraph: &ParagraphBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "struck",
							Text: &TextContent{
								Content: "struck",
							},
							Annotations: &TextAnnotations{
								Strikethrough: true,
							},
						},
					},
				},
			},
			expected: "~~struck~~",
		},
		{
			name: "bold and italic",
			block: &Block{
				Type: "paragraph",
				Paragraph: &ParagraphBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "bold italic",
							Text: &TextContent{
								Content: "bold italic",
							},
							Annotations: &TextAnnotations{
								Bold:   true,
								Italic: true,
							},
						},
					},
				},
			},
			expected: "***bold italic**",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertBlockToMarkdown(tt.block)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Tests for links in rich text
func TestRichTextLinks(t *testing.T) {
	tests := []struct {
		name     string
		block    *Block
		expected string
	}{
		{
			name: "text link",
			block: &Block{
				Type: "paragraph",
				Paragraph: &ParagraphBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Google",
							Text: &TextContent{
								Content: "Google",
								Link:    func(s string) *string { return &s }("https://google.com"),
							},
						},
					},
				},
			},
			expected: "[Google](https://google.com)",
		},
		{
			name: "link with href",
			block: &Block{
				Type: "paragraph",
				Paragraph: &ParagraphBlock{
					RichText: []RichText{
						{
							Type:      "text",
							PlainText: "Click here",
							Text: &TextContent{
								Content: "Click here",
							},
							Href: func(s string) *string { return &s }("https://example.com"),
						},
					},
				},
			},
			expected: "[Click here](https://example.com)",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertBlockToMarkdown(tt.block)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Tests for mixed content
func TestMixedContent(t *testing.T) {
	tests := []struct {
		name     string
		blocks   []*Block
		expected string
	}{
		{
			name: "heading and paragraph",
			blocks: []*Block{
				{
					Type: "heading_1",
					Heading1: &Heading1Block{
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "Title",
								Text: &TextContent{
									Content: "Title",
								},
							},
						},
					},
				},
				{
					Type: "paragraph",
					Paragraph: &ParagraphBlock{
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "This is content",
								Text: &TextContent{
									Content: "This is content",
								},
							},
						},
					},
				},
			},
			expected: "# Title\nThis is content",
		},
		{
			name: "paragraph and list",
			blocks: []*Block{
				{
					Type: "paragraph",
					Paragraph: &ParagraphBlock{
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "Items:",
								Text: &TextContent{
									Content: "Items:",
								},
							},
						},
					},
				},
				{
					Type: "bulleted_list_item",
					BulletedList: &BulletedListBlock{
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "Item 1",
								Text: &TextContent{
									Content: "Item 1",
								},
							},
						},
					},
				},
				{
					Type: "bulleted_list_item",
					BulletedList: &BulletedListBlock{
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "Item 2",
								Text: &TextContent{
									Content: "Item 2",
								},
							},
						},
					},
				},
			},
			expected: "Items:\n- Item 1\n- Item 2",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToMarkdown(tt.blocks)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Tests for edge cases
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "nil block",
			input:    ([]*Block)(nil),
			expected: "",
		},
		{
			name:     "empty block slice",
			input:    []*Block{},
			expected: "",
		},
		{
			name: "nil block in slice",
			input: []*Block{
				nil,
				{
					Type: "paragraph",
					Paragraph: &ParagraphBlock{
						RichText: []RichText{
							{
								Type:      "text",
								PlainText: "content",
								Text: &TextContent{
									Content: "content",
								},
							},
						},
					},
				},
			},
			expected: "content",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			var err error

			if blocks, ok := tt.input.([]*Block); ok {
				result, err = converter.ConvertToMarkdown(blocks)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Tests for mentions
func TestMentions(t *testing.T) {
	tests := []struct {
		name     string
		richText RichText
		expected string
	}{
		{
			name: "user mention",
			richText: RichText{
				Type: "mention",
				Mention: &Mention{
					Type: "user",
					User: &User{
						ID:   "user123",
						Name: func(s string) *string { return &s }("John"),
					},
				},
				PlainText: "@John",
			},
			expected: "@John",
		},
		{
			name: "page mention",
			richText: RichText{
				Type: "mention",
				Mention: &Mention{
					Type: "page",
					Page: &PageRef{
						ID: "page123",
					},
				},
				PlainText: "@page123",
			},
			expected: "@page123",
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger).(*markdownConverter)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.formatRichText(tt.richText)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkConvertToMarkdown(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	converter := NewConverter(logger)

	blocks := make([]*Block, 100)
	for i := 0; i < 100; i++ {
		blocks[i] = &Block{
			Type: "paragraph",
			Paragraph: &ParagraphBlock{
				RichText: []RichText{
					{
						Type:      "text",
						PlainText: "Sample paragraph content",
						Text: &TextContent{
							Content: "Sample paragraph content",
						},
					},
				},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = converter.ConvertToMarkdown(blocks)
	}
}
