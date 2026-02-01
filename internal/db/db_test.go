// ABOUTME: Tests for database operations.
// ABOUTME: Covers QuerySent, QueryMessages, and persistence operations.

//go:build !windows

package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQuerySent_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	records, err := store.QuerySent(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QuerySent failed: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestQuerySent_WithRecords(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Insert test records
	now := time.Now()
	testRecords := []SentRecord{
		{Message: "First message", Title: "Title1", Device: "all", Priority: 0, SentAt: now.Add(-2 * time.Hour), RequestID: "req1"},
		{Message: "Second message", Title: "Title2", Device: "phone", Priority: 1, SentAt: now.Add(-1 * time.Hour), RequestID: "req2"},
		{Message: "Third message", Title: "Alert", Device: "all", Priority: 2, SentAt: now, RequestID: "req3"},
	}

	for _, rec := range testRecords {
		if err := store.LogSent(ctx, rec); err != nil {
			t.Fatalf("failed to log sent: %v", err)
		}
	}

	// Query all
	records, err := store.QuerySent(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QuerySent failed: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}

	// Most recent should be first
	if records[0].Title != "Alert" {
		t.Errorf("expected most recent record first, got title: %s", records[0].Title)
	}
}

func TestQuerySent_WithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Insert 5 records
	for i := range 5 {
		rec := SentRecord{
			Message:   "Message",
			Title:     "Title",
			Device:    "all",
			Priority:  0,
			SentAt:    time.Now().Add(time.Duration(i) * time.Hour),
			RequestID: "req",
		}
		if err := store.LogSent(ctx, rec); err != nil {
			t.Fatalf("failed to log sent: %v", err)
		}
	}

	// Query with limit of 2
	records, err := store.QuerySent(ctx, 2, nil, "")
	if err != nil {
		t.Fatalf("QuerySent failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("expected 2 records with limit, got %d", len(records))
	}
}

func TestQuerySent_WithSince(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	now := time.Now()
	oldRecord := SentRecord{Message: "Old", Title: "Old", Device: "all", Priority: 0, SentAt: now.Add(-48 * time.Hour), RequestID: "old"}
	newRecord := SentRecord{Message: "New", Title: "New", Device: "all", Priority: 0, SentAt: now, RequestID: "new"}

	if err := store.LogSent(ctx, oldRecord); err != nil {
		t.Fatalf("failed to log old: %v", err)
	}
	if err := store.LogSent(ctx, newRecord); err != nil {
		t.Fatalf("failed to log new: %v", err)
	}

	// Query since 24 hours ago
	since := now.Add(-24 * time.Hour)
	records, err := store.QuerySent(ctx, 10, &since, "")
	if err != nil {
		t.Fatalf("QuerySent failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("expected 1 record with since filter, got %d", len(records))
	}

	if len(records) > 0 && records[0].Title != "New" {
		t.Errorf("expected New record, got %s", records[0].Title)
	}
}

func TestQuerySent_WithSearch(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	now := time.Now()
	rec1 := SentRecord{Message: "Hello world", Title: "Greeting", Device: "all", Priority: 0, SentAt: now, RequestID: "r1"}
	rec2 := SentRecord{Message: "Goodbye world", Title: "Farewell", Device: "all", Priority: 0, SentAt: now, RequestID: "r2"}
	rec3 := SentRecord{Message: "Something else", Title: "Other", Device: "all", Priority: 0, SentAt: now, RequestID: "r3"}

	for _, rec := range []SentRecord{rec1, rec2, rec3} {
		if err := store.LogSent(ctx, rec); err != nil {
			t.Fatalf("failed to log: %v", err)
		}
	}

	// Search in message
	records, err := store.QuerySent(ctx, 10, nil, "Hello")
	if err != nil {
		t.Fatalf("QuerySent failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("expected 1 record matching 'Hello', got %d", len(records))
	}

	// Search in title
	records, err = store.QuerySent(ctx, 10, nil, "Farewell")
	if err != nil {
		t.Fatalf("QuerySent failed: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("expected 1 record matching 'Farewell', got %d", len(records))
	}

	// Search matching multiple
	records, err = store.QuerySent(ctx, 10, nil, "world")
	if err != nil {
		t.Fatalf("QuerySent failed: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("expected 2 records matching 'world', got %d", len(records))
	}
}

func TestQuerySent_NilStore(t *testing.T) {
	var store *Store
	_, err := store.QuerySent(context.Background(), 10, nil, "")
	if err == nil {
		t.Error("expected error for nil store")
	}
}

func TestQuerySent_DefaultLimit(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Insert 25 records
	for i := range 25 {
		rec := SentRecord{
			Message:   "Message",
			Title:     "Title",
			Device:    "all",
			Priority:  0,
			SentAt:    time.Now().Add(time.Duration(i) * time.Minute),
			RequestID: "req",
		}
		if err := store.LogSent(ctx, rec); err != nil {
			t.Fatalf("failed to log sent: %v", err)
		}
	}

	// Query with zero limit should default to 20
	records, err := store.QuerySent(ctx, 0, nil, "")
	if err != nil {
		t.Fatalf("QuerySent failed: %v", err)
	}

	if len(records) != 20 {
		t.Errorf("expected 20 records with default limit, got %d", len(records))
	}
}

func TestOpenEmpty(t *testing.T) {
	_, err := Open("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestOpenAndMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestOpenCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nested", "dir", "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Verify nested directories were created
	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Error("nested directories were not created")
	}
}

func TestClose_NilStore(t *testing.T) {
	var store *Store
	err := store.Close()
	if err != nil {
		t.Errorf("Close() on nil store should return nil, got %v", err)
	}
}

func TestClose_NilSQL(t *testing.T) {
	store := &Store{sql: nil}
	err := store.Close()
	if err != nil {
		t.Errorf("Close() with nil sql should return nil, got %v", err)
	}
}

func TestPersistMessages_NilStore(t *testing.T) {
	var store *Store
	_, err := store.PersistMessages(context.Background(), []MessageRecord{{Message: "test"}})
	if err == nil {
		t.Error("expected error for nil store")
	}
}

func TestPersistMessages_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	count, err := store.PersistMessages(ctx, nil)
	if err != nil {
		t.Fatalf("PersistMessages failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	count, err = store.PersistMessages(ctx, []MessageRecord{})
	if err != nil {
		t.Fatalf("PersistMessages failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestPersistMessages_WithSentAt(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()
	sentAt := now.Add(-1 * time.Hour)

	msgs := []MessageRecord{
		{
			PushoverID: 12345,
			Message:    "Test with sent_at",
			ReceivedAt: now,
			SentAt:     &sentAt,
		},
	}

	count, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	// Query and verify
	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].SentAt == nil {
		t.Error("expected SentAt to be set")
	}
}

func TestPersistMessages_ZeroReceivedAt(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	msgs := []MessageRecord{
		{
			PushoverID: 12345,
			Message:    "Test with zero received_at",
			ReceivedAt: time.Time{}, // zero time
		},
	}

	count, err := store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	// Query and verify - should use current time
	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ReceivedAt.IsZero() {
		t.Error("ReceivedAt should not be zero")
	}
}

func TestLogSent_NilStore(t *testing.T) {
	var store *Store
	err := store.LogSent(context.Background(), SentRecord{Message: "test"})
	if err == nil {
		t.Error("expected error for nil store")
	}
}

func TestLogSent_ZeroSentAt(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	rec := SentRecord{
		Message: "Test with zero sent_at",
		SentAt:  time.Time{}, // zero time
	}

	err = store.LogSent(ctx, rec)
	if err != nil {
		t.Fatalf("LogSent failed: %v", err)
	}

	// Query and verify - should use current time
	records, err := store.QuerySent(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QuerySent failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].SentAt.IsZero() {
		t.Error("SentAt should not be zero")
	}
}

func TestQueryMessages_NilStore(t *testing.T) {
	var store *Store
	_, err := store.QueryMessages(context.Background(), 10, nil, "")
	if err == nil {
		t.Error("expected error for nil store")
	}
}

func TestQueryMessages_WithFilters(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()

	// Insert test records
	msgs := []MessageRecord{
		{PushoverID: 1, Message: "Hello world", Title: "Greeting", ReceivedAt: now.Add(-48 * time.Hour)},
		{PushoverID: 2, Message: "Goodbye world", Title: "Farewell", ReceivedAt: now},
		{PushoverID: 3, Message: "Something else", Title: "Other", ReceivedAt: now},
	}
	_, err = store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages failed: %v", err)
	}

	// Test since filter
	since := now.Add(-24 * time.Hour)
	records, err := store.QueryMessages(ctx, 10, &since, "")
	if err != nil {
		t.Fatalf("QueryMessages with since failed: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 recent records, got %d", len(records))
	}

	// Test search filter
	records, err = store.QueryMessages(ctx, 10, nil, "world")
	if err != nil {
		t.Fatalf("QueryMessages with search failed: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records matching 'world', got %d", len(records))
	}

	// Test search in title
	records, err = store.QueryMessages(ctx, 10, nil, "Greeting")
	if err != nil {
		t.Fatalf("QueryMessages with title search failed: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record matching 'Greeting', got %d", len(records))
	}

	// Test combined filters
	records, err = store.QueryMessages(ctx, 10, &since, "world")
	if err != nil {
		t.Fatalf("QueryMessages with combined filters failed: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 recent record matching 'world', got %d", len(records))
	}
}

func TestQueryMessages_DefaultLimit(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Insert 25 records
	msgs := make([]MessageRecord, 25)
	for i := range 25 {
		msgs[i] = MessageRecord{
			PushoverID: int64(i + 1),
			Message:    "Message",
			ReceivedAt: time.Now().Add(time.Duration(i) * time.Minute),
		}
	}
	_, err = store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages failed: %v", err)
	}

	// Query with zero limit should default to 20
	records, err := store.QueryMessages(ctx, 0, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages failed: %v", err)
	}
	if len(records) != 20 {
		t.Errorf("expected 20 records with default limit, got %d", len(records))
	}
}

func TestQueryMessages_AckedAndHTML(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	msgs := []MessageRecord{
		{PushoverID: 1, Message: "Test", ReceivedAt: time.Now(), Acked: true, HTML: true},
		{PushoverID: 2, Message: "Test2", ReceivedAt: time.Now(), Acked: false, HTML: false},
	}
	_, err = store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages failed: %v", err)
	}

	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages failed: %v", err)
	}

	// Find the record with PushoverID 1
	var rec1, rec2 *MessageRecord
	for i := range records {
		switch records[i].PushoverID {
		case 1:
			rec1 = &records[i]
		case 2:
			rec2 = &records[i]
		}
	}

	if rec1 == nil || rec2 == nil {
		t.Fatal("could not find both records")
	}

	if !rec1.Acked {
		t.Error("expected Acked to be true for record 1")
	}
	if !rec1.HTML {
		t.Error("expected HTML to be true for record 1")
	}
	if rec2.Acked {
		t.Error("expected Acked to be false for record 2")
	}
	if rec2.HTML {
		t.Error("expected HTML to be false for record 2")
	}
}

func TestBoolToInt(t *testing.T) {
	if boolToInt(true) != 1 {
		t.Error("boolToInt(true) should be 1")
	}
	if boolToInt(false) != 0 {
		t.Error("boolToInt(false) should be 0")
	}
}
