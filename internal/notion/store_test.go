package notion

import (
	"database/sql"
	"testing"
	"time"

	"github.com/wesm/msgvault/internal/store"
	"github.com/wesm/msgvault/internal/testutil"
)

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	return testutil.NewTestStore(t)
}

func TestNotionStore_GetOrCreateSource(t *testing.T) {
	st := setupTestStore(t)
	ns := NewNotionStore(st, nil)

	// Create a Notion source
	source, err := ns.GetOrCreateSource("workspace-abc123")
	testutil.MustNoErr(t, err, "GetOrCreateSource")

	if source.ID == 0 {
		t.Error("source ID should be non-zero")
	}
	if source.SourceType != "notion" {
		t.Errorf("SourceType = %q, want %q", source.SourceType, "notion")
	}
	if source.Identifier != "workspace-abc123" {
		t.Errorf("Identifier = %q, want %q", source.Identifier, "workspace-abc123")
	}

	// Get same source again (should return existing)
	source2, err := ns.GetOrCreateSource("workspace-abc123")
	testutil.MustNoErr(t, err, "GetOrCreateSource second call")

	if source2.ID != source.ID {
		t.Errorf("second call ID = %d, want %d", source2.ID, source.ID)
	}
}

func TestNotionStore_UpsertNotionPage(t *testing.T) {
	st := setupTestStore(t)
	ns := NewNotionStore(st, nil)

	source, err := ns.GetOrCreateSource("workspace-test")
	testutil.MustNoErr(t, err, "GetOrCreateSource")

	now := time.Now().Truncate(time.Second)
	page := &PageInfo{
		ID:             "page-abc-123",
		Title:          "My Test Page",
		CreatedTime:    now.Add(-24 * time.Hour),
		LastEditedTime: now,
	}
	markdown := "# My Test Page\n\nThis is the content."
	rawBlocks := []byte(`[{"type":"heading_1","text":"My Test Page"}]`)

	msgID, err := ns.UpsertNotionPage(source.ID, page, markdown, rawBlocks)
	testutil.MustNoErr(t, err, "UpsertNotionPage")

	if msgID == 0 {
		t.Fatal("message ID should be non-zero")
	}

	// Verify message was stored with correct fields
	var subject sql.NullString
	var msgType string
	var sizeEstimate int64
	err = st.DB().QueryRow(
		"SELECT subject, message_type, size_estimate FROM messages WHERE id = ?", msgID,
	).Scan(&subject, &msgType, &sizeEstimate)
	testutil.MustNoErr(t, err, "query message")

	if !subject.Valid || subject.String != "My Test Page" {
		t.Errorf("subject = %v, want %q", subject, "My Test Page")
	}
	if msgType != "notion_page" {
		t.Errorf("message_type = %q, want %q", msgType, "notion_page")
	}
	if sizeEstimate != int64(len(markdown)) {
		t.Errorf("size_estimate = %d, want %d", sizeEstimate, len(markdown))
	}

	// Verify body text was stored
	var bodyText sql.NullString
	err = st.DB().QueryRow(
		"SELECT body_text FROM message_bodies WHERE message_id = ?", msgID,
	).Scan(&bodyText)
	testutil.MustNoErr(t, err, "query body")

	if !bodyText.Valid || bodyText.String != markdown {
		t.Errorf("body_text = %v, want %q", bodyText, markdown)
	}

	// Verify raw data was stored and can be decompressed
	raw, err := st.GetMessageRaw(msgID)
	testutil.MustNoErr(t, err, "GetMessageRaw")

	if string(raw) != string(rawBlocks) {
		t.Errorf("raw data = %q, want %q", raw, rawBlocks)
	}

	// Verify raw format
	var rawFormat string
	err = st.DB().QueryRow(
		"SELECT raw_format FROM message_raw WHERE message_id = ?", msgID,
	).Scan(&rawFormat)
	testutil.MustNoErr(t, err, "query raw format")

	if rawFormat != "notion_markdown" {
		t.Errorf("raw_format = %q, want %q", rawFormat, "notion_markdown")
	}

	// Verify conversation was created
	var convType, convTitle string
	err = st.DB().QueryRow(
		"SELECT conversation_type, title FROM conversations WHERE source_id = ?", source.ID,
	).Scan(&convType, &convTitle)
	testutil.MustNoErr(t, err, "query conversation")

	if convType != "email_thread" {
		// EnsureConversation defaults to 'email_thread'; the source_conversation_id
		// and title convey Notion semantics
		t.Logf("conversation_type = %q (default from EnsureConversation)", convType)
	}
	if convTitle != "Notion Workspace" {
		t.Errorf("conversation title = %q, want %q", convTitle, "Notion Workspace")
	}
}

func TestNotionStore_UpsertNotionPage_Dedup(t *testing.T) {
	st := setupTestStore(t)
	ns := NewNotionStore(st, nil)

	source, err := ns.GetOrCreateSource("workspace-dedup")
	testutil.MustNoErr(t, err, "GetOrCreateSource")

	page := &PageInfo{
		ID:             "page-dedup-1",
		Title:          "Duplicate Test",
		CreatedTime:    time.Now().Add(-time.Hour),
		LastEditedTime: time.Now(),
	}

	// Insert first time
	msgID1, err := ns.UpsertNotionPage(source.ID, page, "Content v1", []byte("raw v1"))
	testutil.MustNoErr(t, err, "UpsertNotionPage first")

	// Insert same page again
	msgID2, err := ns.UpsertNotionPage(source.ID, page, "Content v1", []byte("raw v1"))
	testutil.MustNoErr(t, err, "UpsertNotionPage second")

	// Should return the same message ID (upsert, not duplicate)
	if msgID1 != msgID2 {
		t.Errorf("second insert ID = %d, want %d (same as first)", msgID2, msgID1)
	}

	// Verify only one message exists
	var count int
	err = st.DB().QueryRow(
		"SELECT COUNT(*) FROM messages WHERE source_id = ?", source.ID,
	).Scan(&count)
	testutil.MustNoErr(t, err, "count messages")

	if count != 1 {
		t.Errorf("message count = %d, want 1", count)
	}
}

func TestNotionStore_UpsertNotionPage_Update(t *testing.T) {
	st := setupTestStore(t)
	ns := NewNotionStore(st, nil)

	source, err := ns.GetOrCreateSource("workspace-update")
	testutil.MustNoErr(t, err, "GetOrCreateSource")

	now := time.Now().Truncate(time.Second)
	page := &PageInfo{
		ID:             "page-update-1",
		Title:          "Original Title",
		CreatedTime:    now.Add(-time.Hour),
		LastEditedTime: now,
	}

	// Insert original
	msgID1, err := ns.UpsertNotionPage(source.ID, page, "Original content", []byte("original raw"))
	testutil.MustNoErr(t, err, "UpsertNotionPage original")

	// Update with changed content
	page.Title = "Updated Title"
	page.LastEditedTime = now.Add(time.Minute)
	updatedMarkdown := "Updated content with more text"
	updatedRaw := []byte("updated raw blocks")

	msgID2, err := ns.UpsertNotionPage(source.ID, page, updatedMarkdown, updatedRaw)
	testutil.MustNoErr(t, err, "UpsertNotionPage updated")

	// Same message ID
	if msgID1 != msgID2 {
		t.Errorf("updated ID = %d, want %d", msgID2, msgID1)
	}

	// Verify subject was updated
	var subject sql.NullString
	err = st.DB().QueryRow(
		"SELECT subject FROM messages WHERE id = ?", msgID1,
	).Scan(&subject)
	testutil.MustNoErr(t, err, "query updated subject")

	if !subject.Valid || subject.String != "Updated Title" {
		t.Errorf("subject = %v, want %q", subject, "Updated Title")
	}

	// Verify body was updated
	var bodyText sql.NullString
	err = st.DB().QueryRow(
		"SELECT body_text FROM message_bodies WHERE message_id = ?", msgID1,
	).Scan(&bodyText)
	testutil.MustNoErr(t, err, "query updated body")

	if !bodyText.Valid || bodyText.String != updatedMarkdown {
		t.Errorf("body_text = %v, want %q", bodyText, updatedMarkdown)
	}

	// Verify raw was updated
	raw, err := st.GetMessageRaw(msgID1)
	testutil.MustNoErr(t, err, "GetMessageRaw updated")

	if string(raw) != string(updatedRaw) {
		t.Errorf("raw = %q, want %q", raw, updatedRaw)
	}

	// Verify size estimate was updated
	var sizeEstimate int64
	err = st.DB().QueryRow(
		"SELECT size_estimate FROM messages WHERE id = ?", msgID1,
	).Scan(&sizeEstimate)
	testutil.MustNoErr(t, err, "query updated size")

	if sizeEstimate != int64(len(updatedMarkdown)) {
		t.Errorf("size_estimate = %d, want %d", sizeEstimate, len(updatedMarkdown))
	}
}

func TestNotionStore_SyncCursor(t *testing.T) {
	st := setupTestStore(t)
	ns := NewNotionStore(st, nil)

	source, err := ns.GetOrCreateSource("workspace-cursor")
	testutil.MustNoErr(t, err, "GetOrCreateSource")

	// Set a sync cursor
	cursorTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	err = ns.UpdateSyncCursor(source.ID, cursorTime)
	testutil.MustNoErr(t, err, "UpdateSyncCursor")

	// Retrieve it
	got, err := ns.GetSyncCursor(source.ID)
	testutil.MustNoErr(t, err, "GetSyncCursor")

	if !got.Equal(cursorTime) {
		t.Errorf("GetSyncCursor = %v, want %v", got, cursorTime)
	}

	// Update to a newer cursor
	newerTime := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)
	err = ns.UpdateSyncCursor(source.ID, newerTime)
	testutil.MustNoErr(t, err, "UpdateSyncCursor newer")

	got, err = ns.GetSyncCursor(source.ID)
	testutil.MustNoErr(t, err, "GetSyncCursor newer")

	if !got.Equal(newerTime) {
		t.Errorf("GetSyncCursor = %v, want %v", got, newerTime)
	}
}

func TestNotionStore_SyncCursor_Empty(t *testing.T) {
	st := setupTestStore(t)
	ns := NewNotionStore(st, nil)

	source, err := ns.GetOrCreateSource("workspace-empty-cursor")
	testutil.MustNoErr(t, err, "GetOrCreateSource")

	// No cursor set yet - should return zero time
	got, err := ns.GetSyncCursor(source.ID)
	testutil.MustNoErr(t, err, "GetSyncCursor empty")

	if !got.IsZero() {
		t.Errorf("GetSyncCursor = %v, want zero time", got)
	}
}

func TestNotionStore_EmptyMarkdown(t *testing.T) {
	st := setupTestStore(t)
	ns := NewNotionStore(st, nil)

	source, err := ns.GetOrCreateSource("workspace-empty")
	testutil.MustNoErr(t, err, "GetOrCreateSource")

	page := &PageInfo{
		ID:             "page-empty-1",
		Title:          "Empty Page",
		CreatedTime:    time.Now().Add(-time.Hour),
		LastEditedTime: time.Now(),
	}

	// Insert with empty markdown and no raw blocks
	msgID, err := ns.UpsertNotionPage(source.ID, page, "", nil)
	testutil.MustNoErr(t, err, "UpsertNotionPage empty")

	if msgID == 0 {
		t.Fatal("message ID should be non-zero")
	}

	// Verify body_text is NULL (empty string -> invalid NullString)
	var bodyText sql.NullString
	err = st.DB().QueryRow(
		"SELECT body_text FROM message_bodies WHERE message_id = ?", msgID,
	).Scan(&bodyText)
	testutil.MustNoErr(t, err, "query body")

	if bodyText.Valid {
		t.Errorf("body_text should be NULL for empty markdown, got %q", bodyText.String)
	}

	// Verify no raw data was stored
	var rawCount int
	err = st.DB().QueryRow(
		"SELECT COUNT(*) FROM message_raw WHERE message_id = ?", msgID,
	).Scan(&rawCount)
	testutil.MustNoErr(t, err, "count raw")

	if rawCount != 0 {
		t.Errorf("raw count = %d, want 0 for nil raw blocks", rawCount)
	}

	// Verify size_estimate is 0
	var sizeEstimate int64
	err = st.DB().QueryRow(
		"SELECT size_estimate FROM messages WHERE id = ?", msgID,
	).Scan(&sizeEstimate)
	testutil.MustNoErr(t, err, "query size")

	if sizeEstimate != 0 {
		t.Errorf("size_estimate = %d, want 0", sizeEstimate)
	}
}

func TestNotionStore_Store(t *testing.T) {
	st := setupTestStore(t)
	ns := NewNotionStore(st, nil)

	// Verify Store() returns the underlying store
	if ns.Store() != st {
		t.Error("Store() should return the underlying store")
	}
}

func TestNotionStore_GetSyncCursor_NotFound(t *testing.T) {
	st := setupTestStore(t)
	ns := NewNotionStore(st, nil)

	// Try to get cursor for a non-existent source
	_, err := ns.GetSyncCursor(99999)
	if err == nil {
		t.Error("GetSyncCursor should error for non-existent source")
	}
}
