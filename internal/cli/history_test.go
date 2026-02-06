// ABOUTME: Tests for history command export functionality.
// ABOUTME: Covers markdown, yaml, json formats and file output.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harper/push/internal/storage"
)

func TestWriteHistoryMarkdown(t *testing.T) {
	now := time.Date(2026, 1, 30, 14, 32, 0, 0, time.UTC)
	records := []storage.MessageRecord{
		{
			PushoverID: 12345,
			ReceivedAt: now,
			Title:      "Server Warning",
			Message:    "CPU usage exceeded 90%",
			App:        "monitoring-app",
			Priority:   1,
			URL:        "https://example.com",
		},
		{
			PushoverID: 12346,
			ReceivedAt: now.Add(time.Hour),
			Title:      "",
			Message:    "Simple message",
			App:        "",
			Priority:   0,
		},
	}

	sentRecords := []storage.SentRecord{
		{
			ID:        1,
			SentAt:    now.Add(2 * time.Hour),
			Title:     "Deployment Notice",
			Message:   "Deploying v2.1.0 to production",
			Device:    "all",
			Priority:  0,
			RequestID: "abc123",
		},
	}

	var buf bytes.Buffer
	writeHistoryMarkdown(&buf, records, sentRecords)

	output := buf.String()

	// Check header
	if !strings.Contains(output, "# Push Message History") {
		t.Error("expected markdown header")
	}

	// Check export date
	if !strings.Contains(output, "Export date:") {
		t.Error("expected export date")
	}

	// Check received section
	if !strings.Contains(output, "## Received Messages") {
		t.Error("expected received messages section")
	}

	// Check message content
	if !strings.Contains(output, "Server Warning") {
		t.Error("expected title in output")
	}
	if !strings.Contains(output, "CPU usage exceeded 90%") {
		t.Error("expected message in output")
	}
	if !strings.Contains(output, "monitoring-app") {
		t.Error("expected app in output")
	}
	if !strings.Contains(output, "**Priority**: 1") {
		t.Error("expected priority in output")
	}
	if !strings.Contains(output, "https://example.com") {
		t.Error("expected URL in output")
	}

	// Check sent section
	if !strings.Contains(output, "## Sent Messages") {
		t.Error("expected sent messages section")
	}
	if !strings.Contains(output, "Deployment Notice") {
		t.Error("expected sent title in output")
	}
	if !strings.Contains(output, "Deploying v2.1.0 to production") {
		t.Error("expected sent message in output")
	}
}

func TestWriteHistoryMarkdown_EmptyRecords(t *testing.T) {
	var buf bytes.Buffer
	writeHistoryMarkdown(&buf, nil, nil)

	output := buf.String()
	if !strings.Contains(output, "# Push Message History") {
		t.Error("expected markdown header even with no records")
	}
	if !strings.Contains(output, "No messages") {
		t.Error("expected no messages indicator")
	}
}

func TestWriteHistoryYAML(t *testing.T) {
	now := time.Date(2026, 1, 30, 14, 32, 0, 0, time.UTC)
	records := []storage.MessageRecord{
		{
			PushoverID: 12345,
			ReceivedAt: now,
			Title:      "Test Title",
			Message:    "Test Message",
			App:        "test-app",
			Priority:   1,
		},
	}

	sentRecords := []storage.SentRecord{
		{
			ID:       1,
			SentAt:   now,
			Title:    "Sent Title",
			Message:  "Sent Message",
			Device:   "all",
			Priority: 0,
		},
	}

	var buf bytes.Buffer
	err := writeHistoryYAML(&buf, records, sentRecords, nil, "")
	if err != nil {
		t.Fatalf("writeHistoryYAML failed: %v", err)
	}

	output := buf.String()

	// Check YAML structure
	if !strings.Contains(output, "export:") {
		t.Error("expected export key")
	}
	if !strings.Contains(output, "date:") {
		t.Error("expected date key")
	}
	if !strings.Contains(output, "received:") {
		t.Error("expected received key")
	}
	if !strings.Contains(output, "sent:") {
		t.Error("expected sent key")
	}
	if !strings.Contains(output, "pushover_id: 12345") {
		t.Error("expected pushover_id in output")
	}
	if !strings.Contains(output, "Test Title") {
		t.Error("expected title in output")
	}
}

func TestWriteHistoryYAML_EmptyRecords(t *testing.T) {
	var buf bytes.Buffer
	err := writeHistoryYAML(&buf, nil, nil, nil, "")
	if err != nil {
		t.Fatalf("writeHistoryYAML failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "export:") {
		t.Error("expected export key even with no records")
	}
	if !strings.Contains(output, "received: []") {
		t.Error("expected empty received array")
	}
	if !strings.Contains(output, "sent: []") {
		t.Error("expected empty sent array")
	}
}

func TestWriteHistoryJSON(t *testing.T) {
	now := time.Date(2026, 1, 30, 14, 32, 0, 0, time.UTC)
	records := []storage.MessageRecord{
		{
			PushoverID: 12345,
			ReceivedAt: now,
			Title:      "Test",
			Message:    "Message",
		},
	}

	var buf bytes.Buffer
	err := writeHistoryJSONFull(&buf, records, nil)
	if err != nil {
		t.Fatalf("writeHistoryJSONFull failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "12345") {
		t.Error("expected pushover ID in JSON output")
	}
}

func TestWriteToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.md")

	content := "# Test Content\n\nThis is a test."

	err := writeToFile(outputPath, []byte(content))
	if err != nil {
		t.Fatalf("writeToFile failed: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if string(data) != content {
		t.Errorf("file content mismatch: got %q, want %q", string(data), content)
	}
}

func TestWriteToFile_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "subdir", "output.md")

	content := "# Test Content"

	err := writeToFile(outputPath, []byte(content))
	if err != nil {
		t.Fatalf("writeToFile failed: %v", err)
	}

	// Verify file was created in subdirectory
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("expected file to be created in subdirectory")
	}
}

func TestFormatValidation(t *testing.T) {
	tests := []struct {
		format  string
		wantErr bool
	}{
		{"table", false},
		{"json", false},
		{"markdown", false},
		{"yaml", false},
		{"TABLE", false},
		{"JSON", false},
		{"invalid", true},
		{"", false}, // empty should default to table
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			err := validateFormat(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFormat(%q) error = %v, wantErr %v", tt.format, err, tt.wantErr)
			}
		})
	}
}
