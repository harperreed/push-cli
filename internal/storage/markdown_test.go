// ABOUTME: Tests for the markdown file-based storage backend.
// ABOUTME: Verifies persistence, querying, filtering, and file format correctness.

//go:build !windows

package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMarkdownStore_NewMarkdownStore_EmptyPath(t *testing.T) {
	_, err := NewMarkdownStore("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestMarkdownStore_NewMarkdownStore_CreatesSubdirs(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "push-data")

	store, err := NewMarkdownStore(dataDir)
	if err != nil {
		t.Fatalf("NewMarkdownStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Verify subdirectories were created
	for _, sub := range []string{"received", "sent"} {
		info, err := os.Stat(filepath.Join(dataDir, sub))
		if err != nil {
			t.Errorf("expected %s directory to exist: %v", sub, err)
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", sub)
		}
	}
}

func TestMarkdownStore_Close(t *testing.T) {
	store := newTestMarkdownStore(t)
	err := store.Close()
	if err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestMarkdownStore_PersistMessages_Empty(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	count, err := store.PersistMessages(ctx, nil)
	if err != nil {
		t.Fatalf("PersistMessages(nil) error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	count, err = store.PersistMessages(ctx, []MessageRecord{})
	if err != nil {
		t.Fatalf("PersistMessages([]) error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestMarkdownStore_PersistMessages_Roundtrip(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	sentAt := now.Add(-time.Hour)

	msgs := []MessageRecord{
		{
			PushoverID: 12345,
			UMID:       "umid-abc",
			Title:      "Test Title",
			Message:    "Hello World",
			App:        "test-app",
			AID:        999,
			Icon:       "icon.png",
			ReceivedAt: now,
			SentAt:     &sentAt,
			Priority:   1,
			URL:        "https://example.com",
			Acked:      true,
			HTML:       true,
		},
	}

	count, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	// Verify file was created
	filePath := filepath.Join(store.dataDir, "received", "12345.md")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist", filePath)
	}

	// Verify content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "pushover_id: 12345") {
		t.Error("expected pushover_id in frontmatter")
	}
	if !strings.Contains(content, "Hello World") {
		t.Error("expected message body")
	}
	if !strings.Contains(content, "title: Test Title") {
		t.Error("expected title in frontmatter")
	}
	if !strings.Contains(content, "acked: true") {
		t.Error("expected acked in frontmatter")
	}
}

func TestMarkdownStore_PersistMessages_RoundtripFieldValues(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	sentAt := now.Add(-time.Hour)

	msgs := []MessageRecord{
		{
			PushoverID: 12345,
			UMID:       "umid-abc",
			Title:      "Test Title",
			Message:    "Hello World",
			App:        "test-app",
			AID:        999,
			Icon:       "icon.png",
			ReceivedAt: now,
			SentAt:     &sentAt,
			Priority:   1,
			URL:        "https://example.com",
			Acked:      true,
			HTML:       true,
		},
	}

	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	// Read back through query
	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.PushoverID != 12345 {
		t.Errorf("PushoverID = %d, want 12345", rec.PushoverID)
	}
	if rec.UMID != "umid-abc" {
		t.Errorf("UMID = %q, want %q", rec.UMID, "umid-abc")
	}
	if rec.Title != "Test Title" {
		t.Errorf("Title = %q, want %q", rec.Title, "Test Title")
	}
	if rec.Message != "Hello World" {
		t.Errorf("Message = %q, want %q", rec.Message, "Hello World")
	}
	if rec.App != "test-app" {
		t.Errorf("App = %q, want %q", rec.App, "test-app")
	}
	if rec.Priority != 1 {
		t.Errorf("Priority = %d, want 1", rec.Priority)
	}
	if rec.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", rec.URL, "https://example.com")
	}
	if !rec.Acked {
		t.Error("expected Acked = true")
	}
	if !rec.HTML {
		t.Error("expected HTML = true")
	}
	if rec.SentAt == nil {
		t.Error("expected SentAt to be set")
	}
}

func TestMarkdownStore_PersistMessages_Overwrite(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	msgs := []MessageRecord{
		{PushoverID: 100, Message: "Original", ReceivedAt: now},
	}
	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("first PersistMessages() error: %v", err)
	}

	// Overwrite with updated message
	msgs = []MessageRecord{
		{PushoverID: 100, Message: "Updated", ReceivedAt: now},
	}
	_, err = store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("second PersistMessages() error: %v", err)
	}

	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record after overwrite, got %d", len(records))
	}
	if records[0].Message != "Updated" {
		t.Errorf("Message = %q, want %q", records[0].Message, "Updated")
	}
}

func TestMarkdownStore_LogSent_Roundtrip(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	err := store.LogSent(ctx, SentRecord{
		Message:   "Test sent message",
		Title:     "Sent Title",
		Device:    "phone",
		Priority:  1,
		SentAt:    now,
		RequestID: "req-123-abc",
	})
	if err != nil {
		t.Fatalf("LogSent() error: %v", err)
	}

	// Verify a file was created in the sent directory
	entries, err := os.ReadDir(filepath.Join(store.dataDir, "sent"))
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in sent dir, got %d", len(entries))
	}

	// Query back
	records, err := store.QuerySent(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QuerySent() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Message != "Test sent message" {
		t.Errorf("Message = %q, want %q", rec.Message, "Test sent message")
	}
	if rec.Title != "Sent Title" {
		t.Errorf("Title = %q, want %q", rec.Title, "Sent Title")
	}
	if rec.Device != "phone" {
		t.Errorf("Device = %q, want %q", rec.Device, "phone")
	}
	if rec.Priority != 1 {
		t.Errorf("Priority = %d, want 1", rec.Priority)
	}
	if rec.RequestID != "req-123-abc" {
		t.Errorf("RequestID = %q, want %q", rec.RequestID, "req-123-abc")
	}
}

func TestMarkdownStore_QueryMessages_SinceFilter(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	msgs := []MessageRecord{
		{PushoverID: 1, Message: "Old", ReceivedAt: now.Add(-48 * time.Hour)},
		{PushoverID: 2, Message: "Recent", ReceivedAt: now},
	}
	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	since := now.Add(-24 * time.Hour)
	records, err := store.QueryMessages(ctx, 10, &since, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 recent record, got %d", len(records))
	}
	if len(records) > 0 && records[0].Message != "Recent" {
		t.Errorf("expected 'Recent', got %q", records[0].Message)
	}
}

func TestMarkdownStore_QueryMessages_SearchFilter(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	msgs := []MessageRecord{
		{PushoverID: 1, Message: "Hello world", ReceivedAt: now},
		{PushoverID: 2, Message: "Goodbye world", ReceivedAt: now},
		{PushoverID: 3, Message: "Something else", Title: "Hello title", ReceivedAt: now},
	}
	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	// Search in message body
	records, err := store.QueryMessages(ctx, 10, nil, "world")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records matching 'world', got %d", len(records))
	}

	// Search in title
	records, err = store.QueryMessages(ctx, 10, nil, "Hello")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records matching 'Hello' (msg + title), got %d", len(records))
	}
}

func TestMarkdownStore_QueryMessages_SearchCaseInsensitive(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	msgs := []MessageRecord{
		{PushoverID: 1, Message: "Hello World", ReceivedAt: now},
	}
	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	records, err := store.QueryMessages(ctx, 10, nil, "hello")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record matching 'hello' (case insensitive), got %d", len(records))
	}
}

func TestMarkdownStore_QueryMessages_Limit(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	msgs := make([]MessageRecord, 10)
	for i := range msgs {
		msgs[i] = MessageRecord{
			PushoverID: int64(i + 1),
			Message:    "Message",
			ReceivedAt: now.Add(time.Duration(i) * time.Minute),
		}
	}
	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	records, err := store.QueryMessages(ctx, 3, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}
}

func TestMarkdownStore_QueryMessages_DefaultLimit(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	msgs := make([]MessageRecord, 25)
	for i := range msgs {
		msgs[i] = MessageRecord{
			PushoverID: int64(i + 1),
			Message:    "Message",
			ReceivedAt: now.Add(time.Duration(i) * time.Minute),
		}
	}
	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	records, err := store.QueryMessages(ctx, 0, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 20 {
		t.Errorf("expected 20 records (default limit), got %d", len(records))
	}
}

func TestMarkdownStore_QueryMessages_OrderByReceivedAtDesc(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	msgs := []MessageRecord{
		{PushoverID: 1, Message: "First", ReceivedAt: now.Add(-2 * time.Hour)},
		{PushoverID: 2, Message: "Second", ReceivedAt: now.Add(-1 * time.Hour)},
		{PushoverID: 3, Message: "Third", ReceivedAt: now},
	}
	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	// Most recent first
	if records[0].PushoverID != 3 {
		t.Errorf("expected most recent first (PushoverID 3), got %d", records[0].PushoverID)
	}
	if records[2].PushoverID != 1 {
		t.Errorf("expected oldest last (PushoverID 1), got %d", records[2].PushoverID)
	}
}

func TestMarkdownStore_QuerySent_SinceFilter(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	_ = store.LogSent(ctx, SentRecord{Message: "Old", SentAt: now.Add(-48 * time.Hour)})
	_ = store.LogSent(ctx, SentRecord{Message: "Recent", SentAt: now})

	since := now.Add(-24 * time.Hour)
	records, err := store.QuerySent(ctx, 10, &since, "")
	if err != nil {
		t.Fatalf("QuerySent() error: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestMarkdownStore_QuerySent_SearchFilter(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	_ = store.LogSent(ctx, SentRecord{Message: "Hello world", SentAt: now})
	_ = store.LogSent(ctx, SentRecord{Message: "Goodbye world", SentAt: now.Add(time.Second)})
	_ = store.LogSent(ctx, SentRecord{Message: "Something else", Title: "Hello title", SentAt: now.Add(2 * time.Second)})

	records, err := store.QuerySent(ctx, 10, nil, "world")
	if err != nil {
		t.Fatalf("QuerySent() error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records matching 'world', got %d", len(records))
	}
}

func TestMarkdownStore_QuerySent_OrderBySentAtDesc(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	_ = store.LogSent(ctx, SentRecord{Message: "First", Title: "T1", SentAt: now.Add(-2 * time.Hour)})
	_ = store.LogSent(ctx, SentRecord{Message: "Second", Title: "T2", SentAt: now.Add(-1 * time.Hour)})
	_ = store.LogSent(ctx, SentRecord{Message: "Third", Title: "T3", SentAt: now})

	records, err := store.QuerySent(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QuerySent() error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	// Most recent first
	if records[0].Title != "T3" {
		t.Errorf("expected most recent first (T3), got %q", records[0].Title)
	}
}

func TestMarkdownStore_PersistMessages_ZeroReceivedAt(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	msgs := []MessageRecord{
		{PushoverID: 1, Message: "Test", ReceivedAt: time.Time{}},
	}
	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ReceivedAt.IsZero() {
		t.Error("ReceivedAt should default to now, not zero")
	}
}

func TestMarkdownStore_LogSent_ZeroSentAt(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	err := store.LogSent(ctx, SentRecord{Message: "Test", SentAt: time.Time{}})
	if err != nil {
		t.Fatalf("LogSent() error: %v", err)
	}

	records, err := store.QuerySent(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QuerySent() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].SentAt.IsZero() {
		t.Error("SentAt should default to now, not zero")
	}
}

func TestMarkdownStore_QueryMessages_NoReceivedDir(t *testing.T) {
	tmpDir := t.TempDir()
	store := &MarkdownStore{dataDir: tmpDir}

	// No received subdirectory exists
	ctx := context.Background()
	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if records != nil {
		t.Errorf("expected nil records, got %v", records)
	}
}

func TestMarkdownStore_QuerySent_NoSentDir(t *testing.T) {
	tmpDir := t.TempDir()
	store := &MarkdownStore{dataDir: tmpDir}

	// No sent subdirectory exists
	ctx := context.Background()
	records, err := store.QuerySent(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QuerySent() error: %v", err)
	}
	if records != nil {
		t.Errorf("expected nil records, got %v", records)
	}
}

func TestMarkdownStore_QueryMessages_SkipsMalformedFiles(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// Write a valid message
	msgs := []MessageRecord{
		{PushoverID: 1, Message: "Valid", ReceivedAt: now},
	}
	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	// Write a malformed file
	malformedPath := filepath.Join(store.dataDir, "received", "bad.md")
	if err := os.WriteFile(malformedPath, []byte("no frontmatter here"), 0o600); err != nil {
		t.Fatalf("failed to write malformed file: %v", err)
	}

	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	// Should skip the malformed file and return only the valid one
	if len(records) != 1 {
		t.Errorf("expected 1 valid record (skipping malformed), got %d", len(records))
	}
}

func TestMarkdownStore_MultipleMessages(t *testing.T) {
	store := newTestMarkdownStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	msgs := []MessageRecord{
		{PushoverID: 1, Message: "First", ReceivedAt: now.Add(-2 * time.Minute)},
		{PushoverID: 2, Message: "Second", ReceivedAt: now.Add(-time.Minute)},
		{PushoverID: 3, Message: "Third", ReceivedAt: now},
	}
	count, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 inserted, got %d", count)
	}

	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}
}

// newTestMarkdownStore creates a temporary MarkdownStore for testing.
func newTestMarkdownStore(t *testing.T) *MarkdownStore {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "push-data")
	store, err := NewMarkdownStore(dataDir)
	if err != nil {
		t.Fatalf("NewMarkdownStore() error: %v", err)
	}
	return store
}
