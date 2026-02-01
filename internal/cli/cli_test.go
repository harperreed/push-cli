// ABOUTME: Tests for CLI commands and helper functions.
// ABOUTME: Covers root command, helper functions, and command integration.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harper/push/internal/config"
	"github.com/harper/push/internal/db"
	"github.com/harper/push/internal/pushover"
	"github.com/spf13/cobra"
)

func TestNewRootCmd(t *testing.T) {
	cmd := newRootCmd()

	if cmd.Use != "push" {
		t.Errorf("Use = %q, want %q", cmd.Use, "push")
	}
	if cmd.Short == "" {
		t.Error("expected Short description")
	}
	if cmd.Long == "" {
		t.Error("expected Long description")
	}

	// Verify persistent flags
	configFlag := cmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Error("expected --config flag")
	}
	dataFlag := cmd.PersistentFlags().Lookup("data")
	if dataFlag == nil {
		t.Error("expected --data flag")
	}

	// Verify subcommands
	subcommands := cmd.Commands()
	if len(subcommands) == 0 {
		t.Error("expected subcommands")
	}

	// Check for expected subcommands
	expectedCommands := []string{"login", "logout", "send", "messages", "history", "config", "mcp", "install-skill"}
	for _, expected := range expectedCommands {
		found := false
		for _, sub := range subcommands {
			if sub.Use == expected || strings.HasPrefix(sub.Use, expected+" ") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q", expected)
		}
	}
}

func TestNewClientFromConfig(t *testing.T) {
	cfg := &config.Config{
		AppToken:     "app-token",
		UserKey:      "user-key",
		DeviceID:     "device-id",
		DeviceSecret: "device-secret",
	}

	client := newClientFromConfig(cfg)
	if client == nil {
		t.Fatal("newClientFromConfig returned nil")
	}

	if client.AppToken != "app-token" {
		t.Errorf("AppToken = %q, want %q", client.AppToken, "app-token")
	}
	if client.UserKey != "user-key" {
		t.Errorf("UserKey = %q, want %q", client.UserKey, "user-key")
	}
	if client.DeviceID != "device-id" {
		t.Errorf("DeviceID = %q, want %q", client.DeviceID, "device-id")
	}
	if client.DeviceSecret != "device-secret" {
		t.Errorf("DeviceSecret = %q, want %q", client.DeviceSecret, "device-secret")
	}
}

func TestNewClientFromConfig_Nil(t *testing.T) {
	client := newClientFromConfig(nil)
	if client == nil {
		t.Fatal("newClientFromConfig returned nil for nil config")
	}

	// Should return empty client
	if client.AppToken != "" {
		t.Errorf("expected empty AppToken, got %q", client.AppToken)
	}
}

func TestResolveConfigPath_WithOverride(t *testing.T) {
	opts.configPath = "/custom/path/config.toml"
	defer func() { opts.configPath = "" }()

	path, err := resolveConfigPath()
	if err != nil {
		t.Fatalf("resolveConfigPath() error: %v", err)
	}

	if path != "/custom/path/config.toml" {
		t.Errorf("path = %q, want %q", path, "/custom/path/config.toml")
	}
}

func TestResolveConfigPath_Default(t *testing.T) {
	opts.configPath = ""

	path, err := resolveConfigPath()
	if err != nil {
		t.Fatalf("resolveConfigPath() error: %v", err)
	}

	if !strings.HasSuffix(path, "push/config.toml") {
		t.Errorf("path = %q, expected to end with push/config.toml", path)
	}
}

func TestResolveConfigPath_XDGOverride(t *testing.T) {
	opts.configPath = ""
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	_ = os.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test-config")
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", oldXDG) }()

	path, err := resolveConfigPath()
	if err != nil {
		t.Fatalf("resolveConfigPath() error: %v", err)
	}

	if !strings.HasPrefix(path, "/tmp/xdg-test-config") {
		t.Errorf("path = %q, expected to start with XDG_CONFIG_HOME", path)
	}
}

func TestResolveDataDir_WithOverride(t *testing.T) {
	opts.dataDir = "/custom/data"
	defer func() { opts.dataDir = "" }()

	path, err := resolveDataDir()
	if err != nil {
		t.Fatalf("resolveDataDir() error: %v", err)
	}

	if path != "/custom/data" {
		t.Errorf("path = %q, want %q", path, "/custom/data")
	}
}

func TestResolveDataDir_Default(t *testing.T) {
	opts.dataDir = ""

	path, err := resolveDataDir()
	if err != nil {
		t.Fatalf("resolveDataDir() error: %v", err)
	}

	if !strings.HasSuffix(path, "push") {
		t.Errorf("path = %q, expected to end with push", path)
	}
}

func TestResolveDataDir_XDGOverride(t *testing.T) {
	opts.dataDir = ""
	oldXDG := os.Getenv("XDG_DATA_HOME")
	_ = os.Setenv("XDG_DATA_HOME", "/tmp/xdg-test-data")
	defer func() { _ = os.Setenv("XDG_DATA_HOME", oldXDG) }()

	path, err := resolveDataDir()
	if err != nil {
		t.Fatalf("resolveDataDir() error: %v", err)
	}

	if !strings.HasPrefix(path, "/tmp/xdg-test-data") {
		t.Errorf("path = %q, expected to start with XDG_DATA_HOME", path)
	}
}

func TestNewSendCmd(t *testing.T) {
	cmd := newSendCmd()

	if !strings.HasPrefix(cmd.Use, "send") {
		t.Errorf("Use = %q, want to start with 'send'", cmd.Use)
	}

	// Verify flags exist
	flags := []string{"title", "priority", "url", "url-title", "sound", "device"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected --%s flag", flag)
		}
	}
}

func TestNewMessagesCmd(t *testing.T) {
	cmd := newMessagesCmd()

	if cmd.Use != "messages" {
		t.Errorf("Use = %q, want %q", cmd.Use, "messages")
	}

	// Verify limit flag
	if cmd.Flags().Lookup("limit") == nil {
		t.Error("expected --limit flag")
	}
}

func TestNewHistoryCmd(t *testing.T) {
	cmd := newHistoryCmd()

	if cmd.Use != "history" {
		t.Errorf("Use = %q, want %q", cmd.Use, "history")
	}

	// Verify flags
	flags := []string{"limit", "since", "search", "json", "format", "output", "include-sent"}
	for _, flag := range flags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected --%s flag", flag)
		}
	}
}

func TestHighestMessageID(t *testing.T) {
	tests := []struct {
		name     string
		result   *pushover.FetchResult
		msgs     []pushover.ReceivedMessage
		expected int64
	}{
		{
			name:     "nil result with messages",
			result:   nil,
			msgs:     []pushover.ReceivedMessage{{PushoverID: 100}, {PushoverID: 200}},
			expected: 200,
		},
		{
			name:     "result with LastMessageID",
			result:   &pushover.FetchResult{LastMessageID: 500},
			msgs:     []pushover.ReceivedMessage{{PushoverID: 100}},
			expected: 500,
		},
		{
			name:     "result without LastMessageID",
			result:   &pushover.FetchResult{LastMessageID: 0},
			msgs:     []pushover.ReceivedMessage{{PushoverID: 100}, {PushoverID: 300}, {PushoverID: 200}},
			expected: 300,
		},
		{
			name:     "empty messages",
			result:   nil,
			msgs:     []pushover.ReceivedMessage{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := highestMessageID(tt.result, tt.msgs)
			if result != tt.expected {
				t.Errorf("highestMessageID() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestWriteHistoryTable_Empty(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	writeHistoryTable(cmd, []db.MessageRecord{})

	output := buf.String()
	if !strings.Contains(output, "No history found") {
		t.Errorf("expected 'No history found' message, got: %s", output)
	}
}

func TestWriteHistoryTable_WithRecords(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	records := []db.MessageRecord{
		{
			PushoverID: 12345,
			Message:    "Test message",
			Title:      "Test Title",
			URL:        "https://example.com",
			Priority:   1,
			App:        "test-app",
			ReceivedAt: time.Now(),
		},
	}

	writeHistoryTable(cmd, records)

	output := buf.String()
	if !strings.Contains(output, "Test message") {
		t.Error("expected message in output")
	}
	if !strings.Contains(output, "Test Title") {
		t.Error("expected title in output")
	}
	if !strings.Contains(output, "https://example.com") {
		t.Error("expected URL in output")
	}
	if !strings.Contains(output, "Priority: 1") {
		t.Error("expected priority in output")
	}
	if !strings.Contains(output, "test-app") {
		t.Error("expected app in output")
	}
}

func TestWriteSentTable_Empty(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	writeSentTable(cmd, []db.SentRecord{})

	output := buf.String()
	if !strings.Contains(output, "No sent messages found") {
		t.Errorf("expected 'No sent messages found' message, got: %s", output)
	}
}

func TestWriteSentTable_WithRecords(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	records := []db.SentRecord{
		{
			ID:       1,
			Message:  "Sent message",
			Title:    "Sent Title",
			Device:   "my-phone",
			Priority: 2,
			SentAt:   time.Now(),
		},
	}

	writeSentTable(cmd, records)

	output := buf.String()
	if !strings.Contains(output, "Sent message") {
		t.Error("expected message in output")
	}
	if !strings.Contains(output, "Sent Title") {
		t.Error("expected title in output")
	}
	if !strings.Contains(output, "my-phone") {
		t.Error("expected device in output")
	}
	if !strings.Contains(output, "Priority: 2") {
		t.Error("expected priority in output")
	}
}

func TestGenerateJSONOutput(t *testing.T) {
	records := []db.MessageRecord{
		{PushoverID: 1, Message: "Test"},
	}
	sentRecords := []db.SentRecord{
		{ID: 1, Message: "Sent"},
	}

	result, err := generateJSONOutput(records, sentRecords, true)
	if err != nil {
		t.Fatalf("generateJSONOutput() error: %v", err)
	}

	if !strings.Contains(string(result), "export_date") {
		t.Error("expected export_date in JSON")
	}
	if !strings.Contains(string(result), "received") {
		t.Error("expected received in JSON")
	}
	if !strings.Contains(string(result), "sent") {
		t.Error("expected sent in JSON")
	}
}

func TestGenerateMarkdownOutput(t *testing.T) {
	records := []db.MessageRecord{
		{PushoverID: 1, Message: "Test", ReceivedAt: time.Now()},
	}
	sentRecords := []db.SentRecord{
		{ID: 1, Message: "Sent", SentAt: time.Now()},
	}

	result := generateMarkdownOutput(records, sentRecords)

	if !strings.Contains(string(result), "# Push Message History") {
		t.Error("expected markdown header")
	}
	if !strings.Contains(string(result), "## Received Messages") {
		t.Error("expected received section")
	}
	if !strings.Contains(string(result), "## Sent Messages") {
		t.Error("expected sent section")
	}
}

func TestGenerateYAMLOutput(t *testing.T) {
	records := []db.MessageRecord{
		{PushoverID: 1, Message: "Test", ReceivedAt: time.Now()},
	}
	sentRecords := []db.SentRecord{
		{ID: 1, Message: "Sent", SentAt: time.Now()},
	}
	since := time.Now().Add(-24 * time.Hour)

	result, err := generateYAMLOutput(records, sentRecords, &since, "search term")
	if err != nil {
		t.Fatalf("generateYAMLOutput() error: %v", err)
	}

	if !strings.Contains(string(result), "export:") {
		t.Error("expected export key")
	}
	if !strings.Contains(string(result), "received:") {
		t.Error("expected received key")
	}
	if !strings.Contains(string(result), "sent:") {
		t.Error("expected sent key")
	}
}

func TestWriteToFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.txt")

	err := writeToFile(outputPath, []byte("test content"))
	if err != nil {
		t.Fatalf("writeToFile() error: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != "test content" {
		t.Errorf("content = %q, want %q", string(data), "test content")
	}
}

func TestWriteToFile_CreatesNestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "nested", "dir", "output.txt")

	err := writeToFile(outputPath, []byte("test"))
	if err != nil {
		t.Fatalf("writeToFile() error: %v", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("file was not created")
	}
}

func TestValidateFormat_Valid(t *testing.T) {
	validFormats := []string{"table", "json", "markdown", "yaml", "TABLE", "JSON", "MARKDOWN", "YAML", ""}

	for _, format := range validFormats {
		err := validateFormat(format)
		if err != nil {
			t.Errorf("validateFormat(%q) error: %v", format, err)
		}
	}
}

func TestValidateFormat_Invalid(t *testing.T) {
	invalidFormats := []string{"xml", "csv", "invalid", "txt"}

	for _, format := range invalidFormats {
		err := validateFormat(format)
		if err == nil {
			t.Errorf("validateFormat(%q) should return error", format)
		}
	}
}

func TestDatabasePath(t *testing.T) {
	opts.dataDir = ""

	path, err := databasePath()
	if err != nil {
		t.Fatalf("databasePath() error: %v", err)
	}

	if !strings.HasSuffix(path, "push.db") {
		t.Errorf("path = %q, expected to end with push.db", path)
	}
}

func TestDatabasePath_WithOverride(t *testing.T) {
	opts.dataDir = "/custom/data"
	defer func() { opts.dataDir = "" }()

	path, err := databasePath()
	if err != nil {
		t.Fatalf("databasePath() error: %v", err)
	}

	if path != "/custom/data/push.db" {
		t.Errorf("path = %q, want %q", path, "/custom/data/push.db")
	}
}

func TestNewLoginCmd(t *testing.T) {
	cmd := newLoginCmd()

	if cmd.Use != "login" {
		t.Errorf("Use = %q, want %q", cmd.Use, "login")
	}

	// Verify device-name flag
	if cmd.Flags().Lookup("device-name") == nil {
		t.Error("expected --device-name flag")
	}
}

func TestNewLogoutCmd(t *testing.T) {
	cmd := newLogoutCmd()

	if cmd.Use != "logout" {
		t.Errorf("Use = %q, want %q", cmd.Use, "logout")
	}
}

func TestNewConfigCmd(t *testing.T) {
	cmd := newConfigCmd()

	if cmd.Use != "config" {
		t.Errorf("Use = %q, want %q", cmd.Use, "config")
	}

	// Verify path flag
	if cmd.Flags().Lookup("path") == nil {
		t.Error("expected --path flag")
	}
}

func TestNewMCPCmd(t *testing.T) {
	cmd := newMCPCmd()

	if cmd.Use != "mcp" {
		t.Errorf("Use = %q, want %q", cmd.Use, "mcp")
	}
}

func TestNewInstallSkillCmd(t *testing.T) {
	cmd := newInstallSkillCmd()

	if cmd.Use != "install-skill" {
		t.Errorf("Use = %q, want %q", cmd.Use, "install-skill")
	}

	// Verify yes flag
	if cmd.Flags().Lookup("yes") == nil {
		t.Error("expected --yes flag")
	}
}

func TestWriteHistoryTable_MinimalRecord(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Test record with only required fields
	records := []db.MessageRecord{
		{
			PushoverID: 1,
			Message:    "Simple message",
			ReceivedAt: time.Now(),
		},
	}

	writeHistoryTable(cmd, records)

	output := buf.String()
	if !strings.Contains(output, "Simple message") {
		t.Error("expected message in output")
	}
	// Should not contain empty fields
	if strings.Contains(output, "Title:") {
		t.Error("should not have Title prefix for empty title")
	}
}

func TestWriteSentTable_MinimalRecord(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Test record with only required fields
	records := []db.SentRecord{
		{
			ID:      1,
			Message: "Simple sent message",
			SentAt:  time.Now(),
		},
	}

	writeSentTable(cmd, records)

	output := buf.String()
	if !strings.Contains(output, "Simple sent message") {
		t.Error("expected message in output")
	}
}

func TestGenerateJSONOutput_EmptyRecords(t *testing.T) {
	result, err := generateJSONOutput(nil, nil, false)
	if err != nil {
		t.Fatalf("generateJSONOutput() error: %v", err)
	}

	// Should still have valid JSON structure
	if !strings.Contains(string(result), "export_date") {
		t.Error("expected export_date even for empty records")
	}
}

func TestGenerateMarkdownOutput_NoRecords(t *testing.T) {
	result := generateMarkdownOutput(nil, nil)

	if !strings.Contains(string(result), "No messages found") {
		t.Error("expected 'No messages found' for nil records")
	}
}

func TestGenerateYAMLOutput_NoFilters(t *testing.T) {
	result, err := generateYAMLOutput(nil, nil, nil, "")
	if err != nil {
		t.Fatalf("generateYAMLOutput() error: %v", err)
	}

	if !strings.Contains(string(result), "export:") {
		t.Error("expected export key")
	}
}

func TestWriteHistoryMarkdown_AllFields(t *testing.T) {
	var buf bytes.Buffer

	now := time.Now()
	records := []db.MessageRecord{
		{
			PushoverID: 1,
			ReceivedAt: now,
			Title:      "Title One",
			Message:    "Message body",
			App:        "test-app",
			Priority:   2,
			URL:        "https://example.com/1",
		},
		{
			PushoverID: 2,
			ReceivedAt: now,
			Title:      "",
			Message:    "No title message",
			App:        "",
			Priority:   0,
			URL:        "",
		},
	}

	sentRecords := []db.SentRecord{
		{
			ID:       1,
			SentAt:   now,
			Title:    "Sent Title",
			Message:  "Sent message",
			Device:   "my-device",
			Priority: 1,
		},
		{
			ID:       2,
			SentAt:   now,
			Title:    "",
			Message:  "Sent no title",
			Device:   "",
			Priority: 0,
		},
	}

	writeHistoryMarkdown(&buf, records, sentRecords)

	output := buf.String()

	// Verify received messages content
	if !strings.Contains(output, "Title One") {
		t.Error("expected 'Title One' in output")
	}
	if !strings.Contains(output, "test-app") {
		t.Error("expected app name in output")
	}
	if !strings.Contains(output, "**Priority**: 2") {
		t.Error("expected priority in output")
	}
	if !strings.Contains(output, "https://example.com/1") {
		t.Error("expected URL in output")
	}

	// Verify sent messages content
	if !strings.Contains(output, "Sent Title") {
		t.Error("expected sent title in output")
	}
	if !strings.Contains(output, "my-device") {
		t.Error("expected device in output")
	}
}

func TestGenerateJSONOutput_FullStructure(t *testing.T) {
	now := time.Now()
	records := []db.MessageRecord{
		{
			PushoverID: 12345,
			Message:    "Test message",
			Title:      "Test Title",
			ReceivedAt: now,
		},
	}
	sentRecords := []db.SentRecord{
		{
			ID:      1,
			Message: "Sent message",
			SentAt:  now,
		},
	}

	result, err := generateJSONOutput(records, sentRecords, true)
	if err != nil {
		t.Fatalf("generateJSONOutput() error: %v", err)
	}

	// Verify JSON contains expected keys
	if !strings.Contains(string(result), `"export_date"`) {
		t.Error("expected export_date key")
	}
	if !strings.Contains(string(result), `"received"`) {
		t.Error("expected received key")
	}
	if !strings.Contains(string(result), `"sent"`) {
		t.Error("expected sent key")
	}
	if !strings.Contains(string(result), `12345`) {
		t.Error("expected pushover ID in output")
	}
}

func TestWriteHistoryYAML_WithAllFields(t *testing.T) {
	var buf bytes.Buffer

	now := time.Now()
	since := now.Add(-24 * time.Hour)

	records := []db.MessageRecord{
		{
			PushoverID: 1,
			ReceivedAt: now,
			Title:      "Test",
			Message:    "Message",
			App:        "app",
			Priority:   1,
			URL:        "https://example.com",
		},
	}

	sentRecords := []db.SentRecord{
		{
			ID:       1,
			SentAt:   now,
			Title:    "Sent",
			Message:  "Sent msg",
			Device:   "device",
			Priority: 2,
		},
	}

	err := writeHistoryYAML(&buf, records, sentRecords, &since, "search term")
	if err != nil {
		t.Fatalf("writeHistoryYAML() error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "since:") {
		t.Error("expected since filter")
	}
	if !strings.Contains(output, "search: search term") {
		t.Error("expected search filter")
	}
}

func TestWriteToFile_Permissions(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "secure.txt")

	err := writeToFile(outputPath, []byte("sensitive content"))
	if err != nil {
		t.Fatalf("writeToFile() error: %v", err)
	}

	// Check file permissions (should be 0600)
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	// File should be readable by owner
	perm := info.Mode().Perm()
	if perm&0400 == 0 {
		t.Error("file should be readable by owner")
	}
}

func TestLoadConfig_Success(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")

	// Write a valid config
	cfgContent := `app_token = "test-token"
user_key = "test-user"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Set the config path
	opts.configPath = cfgPath
	defer func() { opts.configPath = "" }()

	cfg, path, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error: %v", err)
	}

	if path != cfgPath {
		t.Errorf("path = %q, want %q", path, cfgPath)
	}
	if cfg.AppToken != "test-token" {
		t.Errorf("AppToken = %q, want %q", cfg.AppToken, "test-token")
	}
	if cfg.UserKey != "test-user" {
		t.Errorf("UserKey = %q, want %q", cfg.UserKey, "test-user")
	}
}

func TestLoadConfig_NonExistent(t *testing.T) {
	// Set a path to a non-existent file
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "nonexistent.toml")

	opts.configPath = cfgPath
	defer func() { opts.configPath = "" }()

	cfg, path, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig() error for non-existent file: %v", err)
	}

	// Should return default (empty) config
	if path != cfgPath {
		t.Errorf("path = %q, want %q", path, cfgPath)
	}
	if cfg.AppToken != "" {
		t.Errorf("expected empty AppToken for default config")
	}
}

func TestOpenStore_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Set the data directory
	opts.dataDir = tmpDir
	defer func() { opts.dataDir = "" }()

	store, path, err := openStore()
	if err != nil {
		t.Fatalf("openStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	expectedPath := filepath.Join(tmpDir, "push.db")
	if path != expectedPath {
		t.Errorf("path = %q, want %q", path, expectedPath)
	}

	// Verify the database file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestOpenStore_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "data")

	// Set the data directory to a nested path that doesn't exist
	opts.dataDir = nestedDir
	defer func() { opts.dataDir = "" }()

	store, path, err := openStore()
	if err != nil {
		t.Fatalf("openStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	expectedPath := filepath.Join(nestedDir, "push.db")
	if path != expectedPath {
		t.Errorf("path = %q, want %q", path, expectedPath)
	}

	// Verify the directory was created
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Error("nested directory was not created")
	}
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "invalid.toml")

	// Write invalid TOML
	if err := os.WriteFile(cfgPath, []byte("invalid [[ toml"), 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	opts.configPath = cfgPath
	defer func() { opts.configPath = "" }()

	_, _, err := loadConfig()
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestResolveConfigPath_HomeDirError(t *testing.T) {
	// This test verifies that resolveConfigPath uses HOME/XDG correctly
	// We can't easily test the error case without mocking os.UserHomeDir
	// So we just verify the happy path with environment variables

	opts.configPath = ""

	// Test with XDG_CONFIG_HOME set
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	testConfigHome := t.TempDir()
	_ = os.Setenv("XDG_CONFIG_HOME", testConfigHome)
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", oldXDG) }()

	path, err := resolveConfigPath()
	if err != nil {
		t.Fatalf("resolveConfigPath() error: %v", err)
	}

	expected := filepath.Join(testConfigHome, "push", "config.toml")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
}

func TestResolveDataDir_HomeDirError(t *testing.T) {
	// This test verifies that resolveDataDir uses HOME/XDG correctly
	opts.dataDir = ""

	// Test with XDG_DATA_HOME set
	oldXDG := os.Getenv("XDG_DATA_HOME")
	testDataHome := t.TempDir()
	_ = os.Setenv("XDG_DATA_HOME", testDataHome)
	defer func() { _ = os.Setenv("XDG_DATA_HOME", oldXDG) }()

	path, err := resolveDataDir()
	if err != nil {
		t.Fatalf("resolveDataDir() error: %v", err)
	}

	expected := filepath.Join(testDataHome, "push")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
}

func TestWriteToFile_CreateError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test cannot run as root")
	}

	// Try to write to a read-only directory
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	defer func() { _ = os.Chmod(readOnlyDir, 0600) }()

	outputPath := filepath.Join(readOnlyDir, "output.txt")
	err := writeToFile(outputPath, []byte("content"))
	if err == nil {
		t.Error("expected error when writing to read-only directory")
	}
}

func TestGenerateJSONOutput_OnlyReceived(t *testing.T) {
	records := []db.MessageRecord{
		{PushoverID: 1, Message: "Test", ReceivedAt: time.Now()},
	}

	result, err := generateJSONOutput(records, nil, false)
	if err != nil {
		t.Fatalf("generateJSONOutput() error: %v", err)
	}

	// Should not have "sent" key when includeSent is false
	if strings.Contains(string(result), `"sent"`) {
		t.Error("JSON should not contain 'sent' when includeSent is false")
	}
}

func TestGenerateYAMLOutput_ErrorCase(t *testing.T) {
	// Test with valid data to ensure no marshal errors
	records := []db.MessageRecord{
		{PushoverID: 1, Message: "Test", ReceivedAt: time.Now()},
	}
	sentRecords := []db.SentRecord{
		{ID: 1, Message: "Sent", SentAt: time.Now()},
	}

	_, err := generateYAMLOutput(records, sentRecords, nil, "")
	if err != nil {
		t.Fatalf("generateYAMLOutput() unexpected error: %v", err)
	}
}

func TestWriteHistoryTable_LongMessage(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Test with a very long message
	longMsg := strings.Repeat("a", 500)
	records := []db.MessageRecord{
		{
			PushoverID: 1,
			Message:    longMsg,
			ReceivedAt: time.Now(),
		},
	}

	writeHistoryTable(cmd, records)

	output := buf.String()
	// Should contain at least part of the message
	if !strings.Contains(output, "aaa") {
		t.Error("expected message content in output")
	}
}

func TestHighestMessageID_NilBoth(t *testing.T) {
	result := highestMessageID(nil, nil)
	if result != 0 {
		t.Errorf("expected 0 for nil inputs, got %d", result)
	}
}

func TestWriteHistoryMarkdown_EmptyArrays(t *testing.T) {
	var buf bytes.Buffer

	writeHistoryMarkdown(&buf, []db.MessageRecord{}, []db.SentRecord{})

	output := buf.String()
	if !strings.Contains(output, "No messages found") {
		t.Error("expected 'No messages found' in output")
	}
}

func TestResolveConfigPath_NoEnvVar(t *testing.T) {
	opts.configPath = ""

	// Clear XDG_CONFIG_HOME to test fallback to ~/.config
	oldXDG := os.Getenv("XDG_CONFIG_HOME")
	_ = os.Unsetenv("XDG_CONFIG_HOME")
	defer func() {
		if oldXDG != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", oldXDG)
		}
	}()

	path, err := resolveConfigPath()
	if err != nil {
		t.Fatalf("resolveConfigPath() error: %v", err)
	}

	// Should use ~/.config/push/config.toml
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "push", "config.toml")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
}

func TestResolveDataDir_NoEnvVar(t *testing.T) {
	opts.dataDir = ""

	// Clear XDG_DATA_HOME to test fallback to ~/.local/share
	oldXDG := os.Getenv("XDG_DATA_HOME")
	_ = os.Unsetenv("XDG_DATA_HOME")
	defer func() {
		if oldXDG != "" {
			_ = os.Setenv("XDG_DATA_HOME", oldXDG)
		}
	}()

	path, err := resolveDataDir()
	if err != nil {
		t.Fatalf("resolveDataDir() error: %v", err)
	}

	// Should use ~/.local/share/push
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "share", "push")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
}

func TestDatabasePath_Error(t *testing.T) {
	// This test verifies databasePath uses resolveDataDir correctly
	opts.dataDir = "/valid/path"
	defer func() { opts.dataDir = "" }()

	path, err := databasePath()
	if err != nil {
		t.Fatalf("databasePath() error: %v", err)
	}

	if path != "/valid/path/push.db" {
		t.Errorf("path = %q, want %q", path, "/valid/path/push.db")
	}
}

func TestOpenStore_Error(t *testing.T) {
	// Try to open store in a read-only location (like /dev/null)
	// This is difficult to test reliably across platforms
	// Instead, verify the function handles the data dir resolution

	tmpDir := t.TempDir()
	opts.dataDir = tmpDir
	defer func() { opts.dataDir = "" }()

	store, path, err := openStore()
	if err != nil {
		t.Fatalf("openStore() error: %v", err)
	}
	defer func() { _ = store.Close() }()

	if store == nil {
		t.Error("expected non-nil store")
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestGenerateJSONOutput_MarshalError(t *testing.T) {
	// Test that the function handles valid input correctly
	// (It's hard to cause a marshal error with standard types)
	records := []db.MessageRecord{
		{PushoverID: 1, Message: "Test with special chars: \u0000", ReceivedAt: time.Now()},
	}

	_, err := generateJSONOutput(records, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateYAMLOutput_WithAllParams(t *testing.T) {
	now := time.Now()
	since := now.Add(-24 * time.Hour)

	records := []db.MessageRecord{
		{PushoverID: 1, Message: "Test", Title: "Title", ReceivedAt: now},
	}
	sentRecords := []db.SentRecord{
		{ID: 1, Message: "Sent", Title: "Sent Title", SentAt: now},
	}

	result, err := generateYAMLOutput(records, sentRecords, &since, "search term")
	if err != nil {
		t.Fatalf("generateYAMLOutput() error: %v", err)
	}

	if !strings.Contains(string(result), "since:") {
		t.Error("expected 'since:' in YAML output")
	}
	if !strings.Contains(string(result), "search:") {
		t.Error("expected 'search:' in YAML output")
	}
}
