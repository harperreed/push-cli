// ABOUTME: Tests for database operations.
// ABOUTME: Covers QuerySent, QueryMessages, and persistence operations.
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
