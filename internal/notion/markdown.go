// Package notion provides type definitions and utilities for the Notion API.
package notion

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
)

// Converter defines the interface for converting Notion blocks to markdown.
type Converter interface {
	ConvertToMarkdown(blocks []*Block) (string, error)
	ConvertBlockToMarkdown(block *Block) (string, error)
}

// markdownConverter implements the Converter interface.
type markdownConverter struct {
	logger *slog.Logger
}

// NewConverter creates a new Markdown converter.
func NewConverter(logger *slog.Logger) Converter {
	if logger == nil {
		logger = slog.Default()
	}
	return &markdownConverter{
		logger: logger,
	}
}

// ConvertToMarkdown converts a slice of blocks to markdown.
// It handles sequential numbering for numbered lists.
func (mc *markdownConverter) ConvertToMarkdown(blocks []*Block) (string, error) {
	if len(blocks) == 0 {
		return "", nil
	}

	var buf bytes.Buffer
	var numberedListCounter int

	for i, block := range blocks {
		if block == nil {
			continue
		}

		// Reset numbered list counter when not in a numbered list block
		if block.Type != "numbered_list_item" {
			numberedListCounter = 0
		}

		markdown, err := mc.convertBlock(block, 0)
		if err != nil {
			mc.logger.Warn(
				"failed to convert block",
				"block_id", block.ID,
				"block_type", block.Type,
				"error", err,
			)
			// Continue processing other blocks
			continue
		}

		// Handle numbered list sequencing
		if block.Type == "numbered_list_item" {
			numberedListCounter++
			// Replace the default numbering (1.) with sequential number
			markdown = strings.Replace(markdown, "1. ", fmt.Sprintf("%d. ", numberedListCounter), 1)
		}

		if markdown != "" {
			buf.WriteString(markdown)
			// Add newline between blocks (except for the last one)
			if i < len(blocks)-1 {
				buf.WriteString("\n")
			}
		}
	}

	return buf.String(), nil
}

// ConvertBlockToMarkdown converts a single block to markdown.
// This is the public convenience method for converting a single block.
func (mc *markdownConverter) ConvertBlockToMarkdown(block *Block) (string, error) {
	if block == nil {
		return "", nil
	}
	return mc.convertBlock(block, 0)
}

// convertBlock is the internal method that converts a block with indentation support.
func (mc *markdownConverter) convertBlock(block *Block, indent int) (string, error) {
	if block == nil {
		return "", nil
	}

	indentStr := strings.Repeat("  ", indent)

	switch block.Type {
	case "paragraph":
		if block.Paragraph == nil {
			return "", nil
		}
		text := mc.richTextToMarkdown(block.Paragraph.RichText)
		if text == "" {
			return "", nil
		}
		return indentStr + text, nil

	case "heading_1":
		if block.Heading1 == nil {
			return "", nil
		}
		text := mc.richTextToMarkdown(block.Heading1.RichText)
		if text == "" {
			return "# ", nil
		}
		return "# " + text, nil

	case "heading_2":
		if block.Heading2 == nil {
			return "", nil
		}
		text := mc.richTextToMarkdown(block.Heading2.RichText)
		if text == "" {
			return "## ", nil
		}
		return "## " + text, nil

	case "heading_3":
		if block.Heading3 == nil {
			return "", nil
		}
		text := mc.richTextToMarkdown(block.Heading3.RichText)
		if text == "" {
			return "### ", nil
		}
		return "### " + text, nil

	case "bulleted_list_item":
		if block.BulletedList == nil {
			return "", nil
		}
		text := mc.richTextToMarkdown(block.BulletedList.RichText)
		if text == "" {
			return indentStr + "- ", nil
		}
		return indentStr + "- " + text, nil

	case "numbered_list_item":
		if block.NumberedList == nil {
			return "", nil
		}
		text := mc.richTextToMarkdown(block.NumberedList.RichText)
		if text == "" {
			return indentStr + "1. ", nil
		}
		return indentStr + "1. " + text, nil

	case "to_do":
		if block.ToDo == nil {
			return "", nil
		}
		text := mc.richTextToMarkdown(block.ToDo.RichText)
		checkbox := "- [ ] "
		if block.ToDo.Checked != nil && *block.ToDo.Checked {
			checkbox = "- [x] "
		}
		if text == "" {
			return indentStr + checkbox, nil
		}
		return indentStr + checkbox + text, nil

	case "code":
		if block.Code == nil {
			return "", nil
		}
		text := mc.richTextToMarkdown(block.Code.RichText)
		lang := ""
		if block.Code.Language != nil {
			lang = *block.Code.Language
		}
		return fmt.Sprintf("```%s\n%s\n```", lang, text), nil

	case "quote":
		if block.Quote == nil {
			return "", nil
		}
		text := mc.richTextToMarkdown(block.Quote.RichText)
		if text == "" {
			return "> ", nil
		}
		// Handle multiline quotes by prefixing each line with >
		lines := strings.Split(text, "\n")
		var quotedLines []string
		for _, line := range lines {
			quotedLines = append(quotedLines, "> "+line)
		}
		return strings.Join(quotedLines, "\n"), nil

	case "callout":
		if block.Callout == nil {
			return "", nil
		}
		text := mc.richTextToMarkdown(block.Callout.RichText)
		emoji := "📌"
		if block.Callout.Icon != nil {
			if block.Callout.Icon.Emoji != nil {
				emoji = *block.Callout.Icon.Emoji
			}
		}
		prefix := fmt.Sprintf("> %s ", emoji)
		if text == "" {
			return indentStr + prefix, nil
		}
		// Handle multiline callouts
		lines := strings.Split(text, "\n")
		var calloutLines []string
		for i, line := range lines {
			if i == 0 {
				calloutLines = append(calloutLines, prefix+line)
			} else {
				calloutLines = append(calloutLines, "> "+line)
			}
		}
		return indentStr + strings.Join(calloutLines, "\n"+indentStr), nil

	case "divider":
		return "---", nil

	case "child_page":
		if block.ChildPage == nil {
			return "", nil
		}
		title := block.ChildPage.Title
		if title == "" {
			return "## Untitled", nil
		}
		return fmt.Sprintf("## %s", title), nil

	case "child_database":
		if block.ChildDatabase == nil {
			return "", nil
		}
		title := block.ChildDatabase.Title
		if title == "" {
			return "## Database", nil
		}
		return fmt.Sprintf("## %s (database)", title), nil

	case "image":
		if block.Image == nil {
			return "", nil
		}
		url, caption := mc.extractFileURL(block.Image.File, block.Image.External, block.Image.Caption)
		if url == "" {
			return "", nil
		}
		if caption != "" {
			return fmt.Sprintf("![%s](%s)", caption, url), nil
		}
		return fmt.Sprintf("![image](%s)", url), nil

	case "video":
		if block.Video == nil {
			return "", nil
		}
		url, caption := mc.extractFileURL(block.Video.File, block.Video.External, block.Video.Caption)
		if url == "" {
			return "", nil
		}
		if caption != "" {
			return fmt.Sprintf("[Video: %s](%s)", caption, url), nil
		}
		return fmt.Sprintf("[Video](%s)", url), nil

	case "file":
		if block.File == nil {
			return "", nil
		}
		url, caption := mc.extractFileURL(block.File.File, block.File.External, block.File.Caption)
		if url == "" {
			return "", nil
		}
		if caption != "" {
			return fmt.Sprintf("[File: %s](%s)", caption, url), nil
		}
		return fmt.Sprintf("[File](%s)", url), nil

	case "bookmark":
		if block.Bookmark == nil {
			return "", nil
		}
		url := block.Bookmark.URL
		caption := mc.richTextToMarkdown(block.Bookmark.Caption)
		if caption != "" {
			return fmt.Sprintf("[%s](%s)", caption, url), nil
		}
		return fmt.Sprintf("[Bookmark](%s)", url), nil

	case "embed":
		if block.Embed == nil {
			return "", nil
		}
		url := block.Embed.URL
		caption := mc.richTextToMarkdown(block.Embed.Caption)
		if caption != "" {
			return fmt.Sprintf("[Embed: %s](%s)", caption, url), nil
		}
		return fmt.Sprintf("[Embed](%s)", url), nil

	case "toggle":
		if block.Toggle == nil {
			return "", nil
		}
		text := mc.richTextToMarkdown(block.Toggle.RichText)
		if text == "" {
			return indentStr + "<toggle></toggle>", nil
		}
		return fmt.Sprintf("%s<toggle>%s</toggle>", indentStr, text), nil

	case "divider_block":
		// Some APIs may use this name
		return "---", nil

	case "synced_block":
		// Synced blocks reference another block, we'll mark them
		if block.SyncedBlock != nil && block.SyncedBlock.SyncedFrom != nil {
			return fmt.Sprintf("(synced from block %s)", block.SyncedBlock.SyncedFrom.BlockID), nil
		}
		return "", nil

	case "table":
		// Tables are complex; mark as placeholder
		if block.Table == nil {
			return "", nil
		}
		return fmt.Sprintf("(table with %d columns)", block.Table.TableWidth), nil

	default:
		mc.logger.Debug("unknown block type", "type", block.Type)
		return "", nil
	}
}

// richTextToMarkdown converts an array of RichText objects to formatted markdown.
func (mc *markdownConverter) richTextToMarkdown(richTexts []RichText) string {
	var buf bytes.Buffer

	for _, rt := range richTexts {
		formatted := mc.formatRichText(rt)
		buf.WriteString(formatted)
	}

	return buf.String()
}

// formatRichText applies markdown formatting to a single RichText object.
func (mc *markdownConverter) formatRichText(rt RichText) string {
	// Start with plain text or content
	var text string
	var url string

	switch rt.Type {
	case "text":
		if rt.Text != nil {
			text = rt.Text.Content
			if rt.Text.Link != nil {
				url = rt.Text.Link.URL
			}
		}
	case "mention":
		if rt.Mention != nil {
			// For mentions, extract contextual info
			switch rt.Mention.Type {
			case "user":
				if rt.Mention.User != nil && rt.Mention.User.Name != nil {
					text = "@" + *rt.Mention.User.Name
				}
			case "page":
				if rt.Mention.Page != nil {
					text = "@" + rt.Mention.Page.ID
				}
			case "database":
				if rt.Mention.Database != nil {
					text = "@" + rt.Mention.Database.ID
				}
			case "date":
				if rt.Mention.Date != nil {
					text = rt.Mention.Date.Start
					if rt.Mention.Date.End != nil {
						text += " to " + *rt.Mention.Date.End
					}
				}
			case "template_mention":
				text = "{{"
				if rt.Mention.TemplateMention != nil {
					text += rt.Mention.TemplateMention.Type
				}
				text += "}}"
			}
		}
	case "equation":
		if rt.Equation != nil {
			text = "$" + rt.Equation.Expression + "$"
		}
	default:
		text = rt.PlainText
	}

	// Fall back to PlainText if text is empty
	if text == "" {
		text = rt.PlainText
	}

	// Apply annotations (formatting)
	if rt.Annotations != nil {
		if rt.Annotations.Code {
			text = "`" + text + "`"
		}
		// Note: order matters for nested formatting
		if rt.Annotations.Bold {
			text = "**" + text + "**"
		}
		if rt.Annotations.Italic {
			text = "*" + text + "*"
		}
		if rt.Annotations.Strikethrough {
			text = "~~" + text + "~~"
		}
		if rt.Annotations.Underline {
			// Markdown doesn't support underline, so use emphasis instead
			text = "__" + text + "__"
		}
	}

	// Apply link if present
	if url != "" {
		text = "[" + text + "](" + url + ")"
	} else if rt.Href != nil {
		text = "[" + text + "](" + *rt.Href + ")"
	}

	return text
}

// extractFileURL extracts URL and caption from file blocks.
func (mc *markdownConverter) extractFileURL(file *FileObject, external *ExternalFile, captions []RichText) (url string, caption string) {
	if file != nil {
		url = file.URL
	} else if external != nil {
		url = external.URL
	}

	if len(captions) > 0 {
		caption = mc.richTextToMarkdown(captions)
	}

	return url, caption
}
