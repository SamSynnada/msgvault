package notion

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/wesm/msgvault/internal/store"
)

// NotionStore wraps the existing Store with Notion-specific helpers.
// It maps Notion pages into the existing message tables using:
//   - sources.source_type = 'notion'
//   - conversations.conversation_type = 'notion_workspace'
//   - messages.message_type = 'notion_page'
//   - message_raw.raw_format = 'notion_markdown'
type NotionStore struct {
	store  *store.Store
	logger *slog.Logger
}

// NewNotionStore creates a new NotionStore.
func NewNotionStore(s *store.Store, logger *slog.Logger) *NotionStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &NotionStore{store: s, logger: logger}
}

// Store returns the underlying store for direct access.
func (ns *NotionStore) Store() *store.Store {
	return ns.store
}

// GetOrCreateSource creates or retrieves a source_type='notion' entry.
func (ns *NotionStore) GetOrCreateSource(workspaceID string) (*store.Source, error) {
	return ns.store.GetOrCreateSource("notion", workspaceID)
}

// UpsertNotionPage maps a Notion page into the messages + message_bodies + message_raw tables.
// Returns the message ID.
func (ns *NotionStore) UpsertNotionPage(sourceID int64, page *PageInfo, markdown string, rawBlocks []byte) (int64, error) {
	// 1. Ensure conversation (one per workspace)
	convID, err := ns.store.EnsureConversation(sourceID, "notion_workspace", "Notion Workspace")
	if err != nil {
		return 0, fmt.Errorf("ensure conversation: %w", err)
	}

	// 2. Upsert message
	msg := &store.Message{
		ConversationID:  convID,
		SourceID:        sourceID,
		SourceMessageID: page.ID,
		MessageType:     "notion_page",
		Subject:         sql.NullString{String: page.Title, Valid: page.Title != ""},
		SentAt:          sql.NullTime{Time: page.CreatedTime, Valid: !page.CreatedTime.IsZero()},
		ReceivedAt:      sql.NullTime{Time: page.LastEditedTime, Valid: !page.LastEditedTime.IsZero()},
		SizeEstimate:    int64(len(markdown)),
	}

	msgID, err := ns.store.UpsertMessage(msg)
	if err != nil {
		return 0, fmt.Errorf("upsert message for page %s: %w", page.ID, err)
	}

	// 3. Store markdown as body text (enables FTS5 search)
	bodyText := sql.NullString{String: markdown, Valid: markdown != ""}
	if err := ns.store.UpsertMessageBody(msgID, bodyText, sql.NullString{}); err != nil {
		return 0, fmt.Errorf("upsert body for page %s: %w", page.ID, err)
	}

	// 4. Store raw blocks as notion_markdown format
	if len(rawBlocks) > 0 {
		if err := ns.store.UpsertMessageRawWithFormat(msgID, rawBlocks, "notion_markdown"); err != nil {
			return 0, fmt.Errorf("upsert raw for page %s: %w", page.ID, err)
		}
	}

	ns.logger.Debug("stored notion page", "page_id", page.ID, "title", page.Title, "msg_id", msgID)
	return msgID, nil
}

// GetSyncCursor retrieves the last sync timestamp from the source's sync_cursor field.
// Returns the zero time if no cursor has been set.
func (ns *NotionStore) GetSyncCursor(sourceID int64) (time.Time, error) {
	source, err := ns.store.GetSourceByID(sourceID)
	if err != nil {
		return time.Time{}, fmt.Errorf("get source %d: %w", sourceID, err)
	}
	if source == nil {
		return time.Time{}, fmt.Errorf("source %d not found", sourceID)
	}
	if !source.SyncCursor.Valid || source.SyncCursor.String == "" {
		return time.Time{}, nil // No previous sync
	}
	t, err := time.Parse(time.RFC3339, source.SyncCursor.String)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse sync cursor %q: %w", source.SyncCursor.String, err)
	}
	return t, nil
}

// UpdateSyncCursor stores the latest last_edited_time as the sync cursor.
func (ns *NotionStore) UpdateSyncCursor(sourceID int64, cursor time.Time) error {
	return ns.store.UpdateSourceSyncCursor(sourceID, cursor.Format(time.RFC3339))
}
