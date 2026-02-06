// ABOUTME: Tests for the SQLite storage backend.
// ABOUTME: Verifies persistence, querying, filtering, and edge cases.

//go:build !windows

package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSqliteStore_NewSqliteStore_EmptyPath(t *testing.T) {
	_, err := NewSqliteStore("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestSqliteStore_NewSqliteStore_CreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nested", "dir", "test.db")

	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSqliteStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()
}

func TestSqliteStore_Close_Nil(t *testing.T) {
	var store *SqliteStore
	err := store.Close()
	if err != nil {
		t.Errorf("Close() on nil should return nil, got %v", err)
	}
}

func TestSqliteStore_PersistMessages_Empty(t *testing.T) {
	store := newTestSqliteStore(t)
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

func TestSqliteStore_PersistMessages_Roundtrip(t *testing.T) {
	store := newTestSqliteStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()
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

func TestSqliteStore_PersistMessages_Upsert(t *testing.T) {
	store := newTestSqliteStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

	msgs := []MessageRecord{
		{PushoverID: 100, Message: "Original", ReceivedAt: now},
	}
	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("first PersistMessages() error: %v", err)
	}

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
		t.Fatalf("expected 1 record after upsert, got %d", len(records))
	}
	if records[0].Message != "Updated" {
		t.Errorf("Message = %q, want %q", records[0].Message, "Updated")
	}
}

func TestSqliteStore_LogSent_Roundtrip(t *testing.T) {
	store := newTestSqliteStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

	err := store.LogSent(ctx, SentRecord{
		Message:   "Test sent",
		Title:     "Title",
		Device:    "phone",
		Priority:  1,
		SentAt:    now,
		RequestID: "req-123",
	})
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

	rec := records[0]
	if rec.Message != "Test sent" {
		t.Errorf("Message = %q, want %q", rec.Message, "Test sent")
	}
	if rec.Title != "Title" {
		t.Errorf("Title = %q, want %q", rec.Title, "Title")
	}
	if rec.Device != "phone" {
		t.Errorf("Device = %q, want %q", rec.Device, "phone")
	}
	if rec.Priority != 1 {
		t.Errorf("Priority = %d, want 1", rec.Priority)
	}
	if rec.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", rec.RequestID, "req-123")
	}
}

func TestSqliteStore_QueryMessages_SinceFilter(t *testing.T) {
	store := newTestSqliteStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

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
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestSqliteStore_QueryMessages_SearchFilter(t *testing.T) {
	store := newTestSqliteStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

	msgs := []MessageRecord{
		{PushoverID: 1, Message: "Hello world", ReceivedAt: now},
		{PushoverID: 2, Message: "Goodbye world", ReceivedAt: now},
		{PushoverID: 3, Message: "Something else", Title: "Hello title", ReceivedAt: now},
	}
	_, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	records, err := store.QueryMessages(ctx, 10, nil, "Hello")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records matching 'Hello', got %d", len(records))
	}
}

func TestSqliteStore_QueryMessages_Limit(t *testing.T) {
	store := newTestSqliteStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

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

func TestSqliteStore_QueryMessages_DefaultLimit(t *testing.T) {
	store := newTestSqliteStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

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

func TestSqliteStore_QuerySent_SinceFilter(t *testing.T) {
	store := newTestSqliteStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

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

func TestSqliteStore_QuerySent_SearchFilter(t *testing.T) {
	store := newTestSqliteStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

	_ = store.LogSent(ctx, SentRecord{Message: "Hello world", SentAt: now})
	_ = store.LogSent(ctx, SentRecord{Message: "Goodbye world", SentAt: now})
	_ = store.LogSent(ctx, SentRecord{Message: "Something else", Title: "Hello title", SentAt: now})

	records, err := store.QuerySent(ctx, 10, nil, "Hello")
	if err != nil {
		t.Fatalf("QuerySent() error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records matching 'Hello', got %d", len(records))
	}
}

func TestSqliteStore_QueryMessages_OrderByReceivedAtDesc(t *testing.T) {
	store := newTestSqliteStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

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
		t.Errorf("expected most recent first, got PushoverID %d", records[0].PushoverID)
	}
}

func TestSqliteStore_QuerySent_OrderBySentAtDesc(t *testing.T) {
	store := newTestSqliteStore(t)
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

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
		t.Errorf("expected most recent first, got title %q", records[0].Title)
	}
}

func TestSqliteStore_PersistMessages_ZeroReceivedAt(t *testing.T) {
	store := newTestSqliteStore(t)
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

func TestSqliteStore_LogSent_ZeroSentAt(t *testing.T) {
	store := newTestSqliteStore(t)
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

// newTestSqliteStore creates a temporary SqliteStore for testing.
func newTestSqliteStore(t *testing.T) *SqliteStore {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSqliteStore() error: %v", err)
	}
	return store
}
