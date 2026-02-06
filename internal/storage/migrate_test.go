// ABOUTME: Tests for data migration between storage backends.
// ABOUTME: Covers SQLite-to-Markdown and Markdown-to-SQLite migration paths.

//go:build !windows

package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrateData_SqliteToMarkdown(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// Create and populate source SQLite store
	src := newTestSqliteStore(t)
	defer func() { _ = src.Close() }()

	msgs := []MessageRecord{
		{PushoverID: 1, Message: "Hello", Title: "T1", App: "app1", ReceivedAt: now.Add(-time.Hour), Priority: 1},
		{PushoverID: 2, Message: "World", Title: "T2", ReceivedAt: now},
	}
	_, err := src.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	sentRecs := []SentRecord{
		{Message: "Sent 1", Title: "ST1", Device: "phone", SentAt: now.Add(-30 * time.Minute), RequestID: "req-1"},
		{Message: "Sent 2", Device: "all", SentAt: now, RequestID: "req-2"},
	}
	for _, rec := range sentRecs {
		if err := src.LogSent(ctx, rec); err != nil {
			t.Fatalf("LogSent() error: %v", err)
		}
	}

	// Create destination markdown store
	dst := newTestMarkdownStore(t)
	defer func() { _ = dst.Close() }()

	// Migrate
	summary, err := MigrateData(ctx, src, dst)
	if err != nil {
		t.Fatalf("MigrateData() error: %v", err)
	}

	if summary.Messages != 2 {
		t.Errorf("Messages = %d, want 2", summary.Messages)
	}
	if summary.Sent != 2 {
		t.Errorf("Sent = %d, want 2", summary.Sent)
	}

	// Verify migrated messages
	records, err := dst.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 migrated messages, got %d", len(records))
	}

	// Verify migrated sent records
	sent, err := dst.QuerySent(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QuerySent() error: %v", err)
	}
	if len(sent) != 2 {
		t.Errorf("expected 2 migrated sent records, got %d", len(sent))
	}
}

func TestMigrateData_MarkdownToSqlite(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// Create and populate source markdown store
	src := newTestMarkdownStore(t)
	defer func() { _ = src.Close() }()

	msgs := []MessageRecord{
		{PushoverID: 10, Message: "Markdown msg 1", Title: "MT1", ReceivedAt: now.Add(-time.Hour)},
		{PushoverID: 20, Message: "Markdown msg 2", ReceivedAt: now},
	}
	_, err := src.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	_ = src.LogSent(ctx, SentRecord{Message: "MD sent", SentAt: now, RequestID: "md-req"})

	// Create destination sqlite store
	dst := newTestSqliteStore(t)
	defer func() { _ = dst.Close() }()

	// Migrate
	summary, err := MigrateData(ctx, src, dst)
	if err != nil {
		t.Fatalf("MigrateData() error: %v", err)
	}

	if summary.Messages != 2 {
		t.Errorf("Messages = %d, want 2", summary.Messages)
	}
	if summary.Sent != 1 {
		t.Errorf("Sent = %d, want 1", summary.Sent)
	}

	// Verify migrated messages in SQLite
	records, err := dst.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 migrated messages, got %d", len(records))
	}

	sent, err := dst.QuerySent(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QuerySent() error: %v", err)
	}
	if len(sent) != 1 {
		t.Errorf("expected 1 migrated sent record, got %d", len(sent))
	}
}

func TestMigrateData_EmptySource(t *testing.T) {
	ctx := context.Background()

	src := newTestSqliteStore(t)
	defer func() { _ = src.Close() }()

	dst := newTestMarkdownStore(t)
	defer func() { _ = dst.Close() }()

	summary, err := MigrateData(ctx, src, dst)
	if err != nil {
		t.Fatalf("MigrateData() error: %v", err)
	}

	if summary.Messages != 0 {
		t.Errorf("Messages = %d, want 0", summary.Messages)
	}
	if summary.Sent != 0 {
		t.Errorf("Sent = %d, want 0", summary.Sent)
	}
}

func TestMigrateData_PreservesMessageFields(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	sentAt := now.Add(-time.Hour)

	src := newTestSqliteStore(t)
	defer func() { _ = src.Close() }()

	msgs := []MessageRecord{
		{
			PushoverID: 999,
			UMID:       "umid-xyz",
			Title:      "Full Fields",
			Message:    "Full message body",
			App:        "full-app",
			AID:        42,
			Icon:       "icon.png",
			ReceivedAt: now,
			SentAt:     &sentAt,
			Priority:   2,
			URL:        "https://example.com/full",
			Acked:      true,
			HTML:       true,
		},
	}
	_, err := src.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("PersistMessages() error: %v", err)
	}

	dst := newTestMarkdownStore(t)
	defer func() { _ = dst.Close() }()

	_, err = MigrateData(ctx, src, dst)
	if err != nil {
		t.Fatalf("MigrateData() error: %v", err)
	}

	records, err := dst.QueryMessages(ctx, 10, nil, "")
	if err != nil {
		t.Fatalf("QueryMessages() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.PushoverID != 999 {
		t.Errorf("PushoverID = %d, want 999", rec.PushoverID)
	}
	if rec.UMID != "umid-xyz" {
		t.Errorf("UMID = %q, want %q", rec.UMID, "umid-xyz")
	}
	if rec.Title != "Full Fields" {
		t.Errorf("Title = %q, want %q", rec.Title, "Full Fields")
	}
	if rec.Message != "Full message body" {
		t.Errorf("Message = %q, want %q", rec.Message, "Full message body")
	}
	if rec.App != "full-app" {
		t.Errorf("App = %q, want %q", rec.App, "full-app")
	}
	if rec.Priority != 2 {
		t.Errorf("Priority = %d, want 2", rec.Priority)
	}
	if rec.URL != "https://example.com/full" {
		t.Errorf("URL = %q, want %q", rec.URL, "https://example.com/full")
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

func TestIsDirNonEmpty_NonExistent(t *testing.T) {
	result, err := IsDirNonEmpty("/nonexistent/path/abc123")
	if err != nil {
		t.Fatalf("IsDirNonEmpty() error: %v", err)
	}
	if result {
		t.Error("expected false for non-existent directory")
	}
}

func TestIsDirNonEmpty_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := IsDirNonEmpty(tmpDir)
	if err != nil {
		t.Fatalf("IsDirNonEmpty() error: %v", err)
	}
	if result {
		t.Error("expected false for empty directory")
	}
}

func TestIsDirNonEmpty_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("content"), 0o600); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	result, err := IsDirNonEmpty(tmpDir)
	if err != nil {
		t.Fatalf("IsDirNonEmpty() error: %v", err)
	}
	if !result {
		t.Error("expected true for non-empty directory")
	}
}

func TestIsDirNonEmpty_IgnoresLockFile(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".lock"), []byte(""), 0o600); err != nil {
		t.Fatalf("failed to create .lock file: %v", err)
	}

	result, err := IsDirNonEmpty(tmpDir)
	if err != nil {
		t.Fatalf("IsDirNonEmpty() error: %v", err)
	}
	if result {
		t.Error("expected false when only .lock file present")
	}
}

func TestIsDirNonEmpty_LockFileWithOtherFiles(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, ".lock"), []byte(""), 0o600); err != nil {
		t.Fatalf("failed to create .lock file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "data.md"), []byte("content"), 0o600); err != nil {
		t.Fatalf("failed to create data file: %v", err)
	}

	result, err := IsDirNonEmpty(tmpDir)
	if err != nil {
		t.Fatalf("IsDirNonEmpty() error: %v", err)
	}
	if !result {
		t.Error("expected true when .lock and other files present")
	}
}
