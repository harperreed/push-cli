// ABOUTME: Tests for message conversion utilities.
// ABOUTME: Covers RecordsFromReceived and PersistReceived functions.
package messages

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/harper/push/internal/pushover"
	"github.com/harper/push/internal/storage"
)

func TestRecordsFromReceived_Empty(t *testing.T) {
	result := RecordsFromReceived(nil)
	if result == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 records, got %d", len(result))
	}

	result = RecordsFromReceived([]pushover.ReceivedMessage{})
	if len(result) != 0 {
		t.Errorf("expected 0 records for empty input, got %d", len(result))
	}
}

func TestRecordsFromReceived_SingleMessage(t *testing.T) {
	msgs := []pushover.ReceivedMessage{
		{
			PushoverID: 12345,
			UMIDStr:    "umid-abc",
			Title:      "Test Title",
			Message:    "Test Message",
			App:        "test-app",
			AID:        999,
			Icon:       "icon.png",
			Date:       1700000000,
			Priority:   1,
			URL:        "https://example.com",
			Acked:      1,
			HTML:       1,
		},
	}

	records := RecordsFromReceived(msgs)
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
	if rec.Message != "Test Message" {
		t.Errorf("Message = %q, want %q", rec.Message, "Test Message")
	}
	if rec.App != "test-app" {
		t.Errorf("App = %q, want %q", rec.App, "test-app")
	}
	if rec.AID != 999 {
		t.Errorf("AID = %d, want 999", rec.AID)
	}
	if rec.Icon != "icon.png" {
		t.Errorf("Icon = %q, want %q", rec.Icon, "icon.png")
	}
	if rec.Priority != 1 {
		t.Errorf("Priority = %d, want 1", rec.Priority)
	}
	if rec.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", rec.URL, "https://example.com")
	}
	if !rec.Acked {
		t.Error("expected Acked to be true")
	}
	if !rec.HTML {
		t.Error("expected HTML to be true")
	}
	if rec.SentAt == nil {
		t.Error("expected SentAt to be set from Date field")
	} else {
		expected := time.Unix(1700000000, 0)
		if !rec.SentAt.Equal(expected) {
			t.Errorf("SentAt = %v, want %v", rec.SentAt, expected)
		}
	}
}

func TestRecordsFromReceived_UMIDFallback(t *testing.T) {
	// When UMIDStr is empty but UMID is set, it should convert to string
	msgs := []pushover.ReceivedMessage{
		{
			PushoverID: 1,
			UMID:       789,
			UMIDStr:    "",
			Message:    "Test",
		},
	}

	records := RecordsFromReceived(msgs)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0].UMID != "789" {
		t.Errorf("UMID = %q, want %q", records[0].UMID, "789")
	}
}

func TestRecordsFromReceived_ZeroDate(t *testing.T) {
	msgs := []pushover.ReceivedMessage{
		{
			PushoverID: 1,
			Message:    "Test",
			Date:       0,
		},
	}

	records := RecordsFromReceived(msgs)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0].SentAt != nil {
		t.Errorf("SentAt should be nil for zero Date, got %v", records[0].SentAt)
	}
}

func TestRecordsFromReceived_AckedAndHTMLZero(t *testing.T) {
	msgs := []pushover.ReceivedMessage{
		{
			PushoverID: 1,
			Message:    "Test",
			Acked:      0,
			HTML:       0,
		},
	}

	records := RecordsFromReceived(msgs)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if records[0].Acked {
		t.Error("expected Acked to be false")
	}
	if records[0].HTML {
		t.Error("expected HTML to be false")
	}
}

func TestRecordsFromReceived_MultipleMessages(t *testing.T) {
	msgs := []pushover.ReceivedMessage{
		{PushoverID: 1, Message: "First"},
		{PushoverID: 2, Message: "Second"},
		{PushoverID: 3, Message: "Third"},
	}

	records := RecordsFromReceived(msgs)
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	for i, rec := range records {
		expectedID := int64(i + 1)
		if rec.PushoverID != expectedID {
			t.Errorf("record[%d].PushoverID = %d, want %d", i, rec.PushoverID, expectedID)
		}
	}
}

func TestPersistReceived_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	count, err := PersistReceived(ctx, store, nil)
	if err != nil {
		t.Fatalf("PersistReceived failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 count, got %d", count)
	}

	count, err = PersistReceived(ctx, store, []pushover.ReceivedMessage{})
	if err != nil {
		t.Fatalf("PersistReceived failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 count for empty slice, got %d", count)
	}
}

func TestPersistReceived_SingleMessage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	msgs := []pushover.ReceivedMessage{
		{
			PushoverID: 12345,
			Title:      "Test",
			Message:    "Hello World",
			App:        "test-app",
			Date:       1700000000,
		},
	}

	count, err := PersistReceived(ctx, store, msgs)
	if err != nil {
		t.Fatalf("PersistReceived failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 count, got %d", count)
	}

	// Verify the message was persisted
	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 persisted record, got %d", len(records))
	}
	if records[0].PushoverID != 12345 {
		t.Errorf("PushoverID = %d, want 12345", records[0].PushoverID)
	}
	if records[0].Message != "Hello World" {
		t.Errorf("Message = %q, want %q", records[0].Message, "Hello World")
	}
}

func TestPersistReceived_MultipleMessages(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	msgs := []pushover.ReceivedMessage{
		{PushoverID: 1, Message: "First"},
		{PushoverID: 2, Message: "Second"},
		{PushoverID: 3, Message: "Third"},
	}

	count, err := PersistReceived(ctx, store, msgs)
	if err != nil {
		t.Fatalf("PersistReceived failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 count, got %d", count)
	}

	// Verify all messages were persisted
	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages failed: %v", err)
	}
	if len(records) != 3 {
		t.Errorf("expected 3 persisted records, got %d", len(records))
	}
}

func TestPersistReceived_Upsert(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewSqliteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()

	// Insert initial message
	msgs := []pushover.ReceivedMessage{
		{PushoverID: 12345, Message: "Original"},
	}
	_, err = PersistReceived(ctx, store, msgs)
	if err != nil {
		t.Fatalf("first PersistReceived failed: %v", err)
	}

	// Upsert with updated content
	msgs = []pushover.ReceivedMessage{
		{PushoverID: 12345, Message: "Updated"},
	}
	count, err := PersistReceived(ctx, store, msgs)
	if err != nil {
		t.Fatalf("second PersistReceived failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 count for upsert, got %d", count)
	}

	// Verify only one record exists with updated content
	records, err := store.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record after upsert, got %d", len(records))
	}
	if records[0].Message != "Updated" {
		t.Errorf("Message = %q, want %q", records[0].Message, "Updated")
	}
}
