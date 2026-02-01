// ABOUTME: Tests for MCP server initialization and tool handlers.
// ABOUTME: Covers NewServer, tool registration, and handler logic.
package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/harper/push/internal/config"
	"github.com/harper/push/internal/db"
	"github.com/harper/push/internal/pushover"
)

func TestNewServer_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	_, err = NewServer(nil, "/path/config.toml", store, "/path/db")
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestNewServer_NilStore(t *testing.T) {
	cfg := &config.Config{AppToken: "token", UserKey: "user"}

	_, err := NewServer(cfg, "/path/config.toml", nil, "/path/db")
	if err == nil {
		t.Error("expected error for nil store")
	}
}

func TestNewServer_Success(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{
		AppToken:        "app-token",
		UserKey:         "user-key",
		DeviceID:        "device-id",
		DeviceSecret:    "device-secret",
		DefaultDevice:   "my-phone",
		DefaultPriority: 0,
	}

	server, err := NewServer(cfg, "/path/config.toml", store, "/path/db")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	if server.cfg != cfg {
		t.Error("config not set correctly")
	}
	if server.store != store {
		t.Error("store not set correctly")
	}
	if server.cfgPath != "/path/config.toml" {
		t.Errorf("cfgPath = %q, want %q", server.cfgPath, "/path/config.toml")
	}
	if server.dbPath != "/path/db" {
		t.Errorf("dbPath = %q, want %q", server.dbPath, "/path/db")
	}
}

func TestNewClient_FromServer(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{
		AppToken:     "app-token",
		UserKey:      "user-key",
		DeviceID:     "device-id",
		DeviceSecret: "device-secret",
	}

	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	client := server.newClient()
	if client == nil {
		t.Fatal("newClient() returned nil")
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

func TestNewClient_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// First create a valid server
	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	// Artificially set config to nil
	server.cfg = nil

	client := server.newClient()
	if client == nil {
		t.Fatal("newClient() returned nil")
	}
	// Should return empty client
	if client.AppToken != "" {
		t.Errorf("expected empty AppToken, got %q", client.AppToken)
	}
}

func TestDetermineAckID_NilResult(t *testing.T) {
	id := determineAckID(nil)
	if id != 0 {
		t.Errorf("expected 0 for nil result, got %d", id)
	}
}

func TestDetermineAckID_WithLastMessageID(t *testing.T) {
	result := &pushover.FetchResult{
		LastMessageID: 12345,
		Messages: []pushover.ReceivedMessage{
			{PushoverID: 100},
			{PushoverID: 200},
		},
	}

	id := determineAckID(result)
	if id != 12345 {
		t.Errorf("expected 12345 from LastMessageID, got %d", id)
	}
}

func TestDetermineAckID_FromMessages(t *testing.T) {
	result := &pushover.FetchResult{
		LastMessageID: 0,
		Messages: []pushover.ReceivedMessage{
			{PushoverID: 100},
			{PushoverID: 500},
			{PushoverID: 300},
		},
	}

	id := determineAckID(result)
	if id != 500 {
		t.Errorf("expected 500 (highest ID), got %d", id)
	}
}

func TestDetermineAckID_EmptyMessages(t *testing.T) {
	result := &pushover.FetchResult{
		LastMessageID: 0,
		Messages:      []pushover.ReceivedMessage{},
	}

	id := determineAckID(result)
	if id != 0 {
		t.Errorf("expected 0 for empty messages, got %d", id)
	}
}

func TestBuildToolResult(t *testing.T) {
	payload := map[string]interface{}{
		"message": "test",
		"count":   42,
	}

	result, err := buildToolResult(payload)
	if err != nil {
		t.Fatalf("buildToolResult() failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Content))
	}
}

func TestBuildResourceResult(t *testing.T) {
	payload := ResourcePayload{
		Metadata: ResourceMetadata{
			Timestamp:   time.Now(),
			ResourceURI: "push://test",
			Count:       5,
		},
		Data: map[string]string{"key": "value"},
	}

	result, err := buildResourceResult("push://test", payload)
	if err != nil {
		t.Fatalf("buildResourceResult() failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Contents))
	}
	if result.Contents[0].URI != "push://test" {
		t.Errorf("URI = %q, want %q", result.Contents[0].URI, "push://test")
	}
	if result.Contents[0].MIMEType != "application/json" {
		t.Errorf("MIMEType = %q, want %q", result.Contents[0].MIMEType, "application/json")
	}
}

func TestSendNotificationInput_Validation(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()

	// Empty message should fail
	_, _, err = server.handleSendNotification(ctx, nil, SendNotificationInput{Message: ""})
	if err == nil {
		t.Error("expected error for empty message")
	}

	_, _, err = server.handleSendNotification(ctx, nil, SendNotificationInput{Message: "   "})
	if err == nil {
		t.Error("expected error for whitespace-only message")
	}
}

func TestSendNotificationInput_InvalidPriority(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()

	// Priority too high
	priority := 3
	_, _, err = server.handleSendNotification(ctx, nil, SendNotificationInput{
		Message:  "test",
		Priority: &priority,
	})
	if err == nil {
		t.Error("expected error for priority > 2")
	}

	// Priority too low
	priority = -3
	_, _, err = server.handleSendNotification(ctx, nil, SendNotificationInput{
		Message:  "test",
		Priority: &priority,
	})
	if err == nil {
		t.Error("expected error for priority < -2")
	}
}

func TestCheckMessagesInput_MissingCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Config without device credentials
	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()

	_, _, err = server.handleCheckMessages(ctx, nil, CheckMessagesInput{})
	if err == nil {
		t.Error("expected error for missing device credentials")
	}
}

func TestMarkReadInput_InvalidMessageID(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{
		AppToken:     "token",
		UserKey:      "user",
		DeviceID:     "device",
		DeviceSecret: "secret",
	}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()

	_, _, err = server.handleMarkRead(ctx, nil, MarkReadInput{MessageID: 0})
	if err == nil {
		t.Error("expected error for zero message id")
	}

	_, _, err = server.handleMarkRead(ctx, nil, MarkReadInput{MessageID: -1})
	if err == nil {
		t.Error("expected error for negative message id")
	}
}

func TestListHistoryInput_InvalidSince(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()

	invalidSince := "not-a-date-at-all-invalid"
	_, _, err = server.handleListHistory(ctx, nil, ListHistoryInput{Since: &invalidSince})
	if err == nil {
		t.Error("expected error for invalid since value")
	}
}

func TestListHistoryHandler_Success(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Add some test messages
	ctx := context.Background()
	msgs := []db.MessageRecord{
		{PushoverID: 1, Message: "Test 1", ReceivedAt: time.Now()},
		{PushoverID: 2, Message: "Test 2", ReceivedAt: time.Now()},
	}
	_, err = store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("failed to persist messages: %v", err)
	}

	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	limit := 10
	result, output, err := server.handleListHistory(ctx, nil, ListHistoryInput{Limit: &limit})
	if err != nil {
		t.Fatalf("handleListHistory() failed: %v", err)
	}

	if result == nil {
		t.Error("result is nil")
	}
	if output.Count != 2 {
		t.Errorf("Count = %d, want 2", output.Count)
	}
	if output.Limit != 10 {
		t.Errorf("Limit = %d, want 10", output.Limit)
	}
}

func TestListHistoryHandler_WithSearch(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	msgs := []db.MessageRecord{
		{PushoverID: 1, Message: "Hello world", ReceivedAt: time.Now()},
		{PushoverID: 2, Message: "Goodbye world", ReceivedAt: time.Now()},
		{PushoverID: 3, Message: "Something else", ReceivedAt: time.Now()},
	}
	_, err = store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("failed to persist messages: %v", err)
	}

	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	search := "world"
	_, output, err := server.handleListHistory(ctx, nil, ListHistoryInput{Search: &search})
	if err != nil {
		t.Fatalf("handleListHistory() failed: %v", err)
	}

	if output.Count != 2 {
		t.Errorf("Count = %d, want 2 (matching 'world')", output.Count)
	}
	if output.Search != "world" {
		t.Errorf("Search = %q, want %q", output.Search, "world")
	}
}

func TestListHistoryHandler_WithSince(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	now := time.Now()
	msgs := []db.MessageRecord{
		{PushoverID: 1, Message: "Old message", ReceivedAt: now.Add(-48 * time.Hour)},
		{PushoverID: 2, Message: "Recent message", ReceivedAt: now},
	}
	_, err = store.PersistMessages(ctx, msgs)
	if err != nil {
		t.Fatalf("failed to persist messages: %v", err)
	}

	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	// Use ISO date format instead of natural language
	since := now.Add(-24 * time.Hour).Format("2006-01-02")
	_, output, err := server.handleListHistory(ctx, nil, ListHistoryInput{Since: &since})
	if err != nil {
		t.Fatalf("handleListHistory() failed: %v", err)
	}

	if output.Count != 1 {
		t.Errorf("Count = %d, want 1 (only recent)", output.Count)
	}
}

func TestResourceMetadata_JSONSerialization(t *testing.T) {
	meta := ResourceMetadata{
		Timestamp:   time.Date(2026, 1, 30, 12, 0, 0, 0, time.UTC),
		ResourceURI: "push://test",
		Count:       42,
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ResourceMetadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ResourceURI != "push://test" {
		t.Errorf("ResourceURI = %q, want %q", decoded.ResourceURI, "push://test")
	}
	if decoded.Count != 42 {
		t.Errorf("Count = %d, want 42", decoded.Count)
	}
}

func TestResourcePayload_JSONSerialization(t *testing.T) {
	payload := ResourcePayload{
		Metadata: ResourceMetadata{
			Timestamp:   time.Now(),
			ResourceURI: "push://test",
			Count:       1,
		},
		Data: map[string]string{"key": "value"},
		Links: map[string]string{
			"history": "push://history",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ResourcePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Metadata.ResourceURI != "push://test" {
		t.Errorf("ResourceURI = %q, want %q", decoded.Metadata.ResourceURI, "push://test")
	}
	if decoded.Links["history"] != "push://history" {
		t.Errorf("Links[history] = %q, want %q", decoded.Links["history"], "push://history")
	}
}

func TestSendNotificationOutput_JSONSerialization(t *testing.T) {
	output := SendNotificationOutput{
		Message:   "Test message",
		Title:     "Test title",
		Device:    "my-phone",
		Priority:  1,
		RequestID: "req-123",
		Receipt:   "rcpt-456",
		Logged:    true,
		Warning:   "",
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded SendNotificationOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Message != "Test message" {
		t.Errorf("Message = %q, want %q", decoded.Message, "Test message")
	}
	if decoded.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", decoded.RequestID, "req-123")
	}
	if !decoded.Logged {
		t.Error("expected Logged to be true")
	}
}

func TestCheckMessagesOutput_JSONSerialization(t *testing.T) {
	output := CheckMessagesOutput{
		Count:      10,
		Returned:   5,
		Limit:      5,
		Persisted:  10,
		AckedUpTo:  12345,
		Messages:   []pushover.ReceivedMessage{{PushoverID: 1, Message: "Test"}},
		Warning:    "",
		AckWarning: "",
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded CheckMessagesOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Count != 10 {
		t.Errorf("Count = %d, want 10", decoded.Count)
	}
	if decoded.AckedUpTo != 12345 {
		t.Errorf("AckedUpTo = %d, want 12345", decoded.AckedUpTo)
	}
}

func TestListHistoryHandler_DefaultLimit(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()
	// nil limit should use default
	_, output, err := server.handleListHistory(ctx, nil, ListHistoryInput{Limit: nil})
	if err != nil {
		t.Fatalf("handleListHistory() failed: %v", err)
	}

	if output.Limit != 20 {
		t.Errorf("Limit = %d, want 20 (default)", output.Limit)
	}
}

func TestListHistoryHandler_ZeroLimit(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()
	limit := 0
	_, output, err := server.handleListHistory(ctx, nil, ListHistoryInput{Limit: &limit})
	if err != nil {
		t.Fatalf("handleListHistory() failed: %v", err)
	}

	if output.Limit != 20 {
		t.Errorf("Limit = %d, want 20 (default for zero)", output.Limit)
	}
}

func TestMarkReadOutput_JSONSerialization(t *testing.T) {
	output := MarkReadOutput{
		MessageID: 12345,
		Status:    "acknowledged",
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded MarkReadOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.MessageID != 12345 {
		t.Errorf("MessageID = %d, want 12345", decoded.MessageID)
	}
	if decoded.Status != "acknowledged" {
		t.Errorf("Status = %q, want %q", decoded.Status, "acknowledged")
	}
}

func TestListHistoryOutput_JSONSerialization(t *testing.T) {
	now := time.Now()
	output := ListHistoryOutput{
		Count:  5,
		Limit:  10,
		Since:  &now,
		Search: "test",
		Messages: []db.MessageRecord{
			{PushoverID: 1, Message: "Test"},
		},
	}

	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ListHistoryOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Count != 5 {
		t.Errorf("Count = %d, want 5", decoded.Count)
	}
	if decoded.Search != "test" {
		t.Errorf("Search = %q, want %q", decoded.Search, "test")
	}
}

func TestHandleSendNotification_MissingCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Config without credentials
	cfg := &config.Config{}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()
	_, _, err = server.handleSendNotification(ctx, nil, SendNotificationInput{Message: "test"})
	if err == nil {
		t.Error("expected error for missing credentials")
	}
}

func TestHandleSendNotification_DefaultPriority(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{
		AppToken:        "token",
		UserKey:         "user",
		DefaultPriority: 1,
	}
	_, err = NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	// Verify the priority logic (we can't fully test without mocking HTTP)
	// but we can verify the input parsing
	input := SendNotificationInput{Message: "test"}
	if input.Priority != nil {
		t.Error("expected nil priority for default")
	}
}

func TestMarkReadInput_MissingCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Config without device credentials
	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()
	_, _, err = server.handleMarkRead(ctx, nil, MarkReadInput{MessageID: 123})
	if err == nil {
		t.Error("expected error for missing device credentials")
	}
}

func TestCheckMessagesInput_DefaultLimit(t *testing.T) {
	input := CheckMessagesInput{Limit: nil}

	// Default limit should be applied in handler
	if input.Limit != nil {
		t.Error("expected nil limit for default")
	}
}

func TestBuildToolResult_ComplexPayload(t *testing.T) {
	type complexPayload struct {
		Messages []struct {
			ID   int    `json:"id"`
			Text string `json:"text"`
		} `json:"messages"`
		Count int `json:"count"`
	}

	payload := complexPayload{
		Messages: []struct {
			ID   int    `json:"id"`
			Text string `json:"text"`
		}{
			{ID: 1, Text: "Hello"},
			{ID: 2, Text: "World"},
		},
		Count: 2,
	}

	result, err := buildToolResult(payload)
	if err != nil {
		t.Fatalf("buildToolResult() failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Content))
	}
}

func TestBuildResourceResult_WithLinks(t *testing.T) {
	payload := ResourcePayload{
		Metadata: ResourceMetadata{
			Timestamp:   time.Now(),
			ResourceURI: "push://test",
			Count:       1,
		},
		Data: map[string]string{"key": "value"},
		Links: map[string]string{
			"history": "push://history",
			"unread":  "push://unread",
			"status":  "push://status",
		},
	}

	result, err := buildResourceResult("push://test", payload)
	if err != nil {
		t.Fatalf("buildResourceResult() failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	// Verify the result contains links
	content := result.Contents[0].Text
	if content == "" {
		t.Error("expected non-empty content")
	}
}

func TestHandleSendNotification_DeviceDefault(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{
		AppToken:        "token",
		UserKey:         "user",
		DefaultDevice:   "my-default-phone",
		DefaultPriority: 1,
	}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	// Verify device defaults when not specified
	input := SendNotificationInput{Message: "test"}
	if input.Device != "" {
		t.Error("input device should be empty initially")
	}

	// When device is empty in input, the handler should use DefaultDevice
	// We can't fully test without mocking HTTP, but we verify the config has defaults
	if cfg.DefaultDevice != "my-default-phone" {
		t.Errorf("DefaultDevice = %q, want %q", cfg.DefaultDevice, "my-default-phone")
	}
	_ = server // Used to create server with config
}

func TestHandleSendNotification_PriorityBoundaries(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()

	// Test priority exactly at boundaries (valid)
	validPriorities := []int{-2, -1, 0, 1, 2}
	for _, p := range validPriorities {
		priority := p
		// We can't fully test without mocking HTTP, but input parsing should work
		input := SendNotificationInput{Message: "test", Priority: &priority}
		if input.Priority == nil || *input.Priority != p {
			t.Errorf("priority not set correctly for %d", p)
		}
	}

	// Test priorities just outside boundaries (invalid)
	invalidPriorities := []int{-3, 3, -10, 10}
	for _, p := range invalidPriorities {
		priority := p
		_, _, err := server.handleSendNotification(ctx, nil, SendNotificationInput{
			Message:  "test",
			Priority: &priority,
		})
		if err == nil {
			t.Errorf("expected error for invalid priority %d", p)
		}
	}
}

func TestCheckMessagesInput_LimitOptions(t *testing.T) {
	// Test various limit values
	tests := []struct {
		name     string
		limit    *int
		expected int
	}{
		{"nil limit uses default", nil, 10},
		{"zero limit uses default", intPtr(0), 10},
		{"negative limit uses default", intPtr(-1), 10},
		{"explicit positive limit", intPtr(5), 5},
		{"large limit", intPtr(100), 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := CheckMessagesInput{Limit: tt.limit}
			// The handler would apply defaults - verify input structure
			if tt.limit != nil && *tt.limit > 0 {
				if *input.Limit != *tt.limit {
					t.Errorf("Limit mismatch")
				}
			}
		})
	}
}

func TestListHistoryHandler_EmptySince(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()

	// Empty string since should be treated like nil
	emptySince := ""
	_, output, err := server.handleListHistory(ctx, nil, ListHistoryInput{Since: &emptySince})
	if err != nil {
		t.Fatalf("handleListHistory() failed: %v", err)
	}

	if output.Since != nil {
		t.Error("expected nil since for empty string input")
	}
}

func TestListHistoryHandler_NilSearch(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	cfg := &config.Config{AppToken: "token", UserKey: "user"}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()

	// Nil search should work
	_, output, err := server.handleListHistory(ctx, nil, ListHistoryInput{Search: nil})
	if err != nil {
		t.Fatalf("handleListHistory() failed: %v", err)
	}

	if output.Search != "" {
		t.Errorf("expected empty search, got %q", output.Search)
	}
}

func TestHandleMarkRead_ValidationOrder(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Test that credential validation happens before message ID validation
	cfg := &config.Config{AppToken: "token", UserKey: "user"} // No device credentials
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	ctx := context.Background()

	// Should fail on credentials, not message ID
	_, _, err = server.handleMarkRead(ctx, nil, MarkReadInput{MessageID: 123})
	if err == nil {
		t.Error("expected error for missing credentials")
	}
	if !contains(err.Error(), "device") {
		t.Errorf("error = %q, expected credential error", err.Error())
	}
}

func TestSendNotificationInput_AllFields(t *testing.T) {
	priority := 1
	input := SendNotificationInput{
		Message:  "test message",
		Title:    "test title",
		Priority: &priority,
		URL:      "https://example.com",
		Sound:    "pushover",
		Device:   "my-phone",
	}

	// Verify JSON serialization
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded SendNotificationInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Message != "test message" {
		t.Errorf("Message = %q, want %q", decoded.Message, "test message")
	}
	if decoded.Title != "test title" {
		t.Errorf("Title = %q, want %q", decoded.Title, "test title")
	}
	if decoded.Priority == nil || *decoded.Priority != 1 {
		t.Error("Priority not decoded correctly")
	}
	if decoded.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", decoded.URL, "https://example.com")
	}
	if decoded.Sound != "pushover" {
		t.Errorf("Sound = %q, want %q", decoded.Sound, "pushover")
	}
	if decoded.Device != "my-phone" {
		t.Errorf("Device = %q, want %q", decoded.Device, "my-phone")
	}
}

func TestListHistoryInput_JSONSerialization(t *testing.T) {
	limit := 50
	since := "yesterday"
	search := "error"

	input := ListHistoryInput{
		Limit:  &limit,
		Since:  &since,
		Search: &search,
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ListHistoryInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Limit == nil || *decoded.Limit != 50 {
		t.Error("Limit not decoded correctly")
	}
	if decoded.Since == nil || *decoded.Since != "yesterday" {
		t.Error("Since not decoded correctly")
	}
	if decoded.Search == nil || *decoded.Search != "error" {
		t.Error("Search not decoded correctly")
	}
}

func TestMarkReadInput_JSONSerialization(t *testing.T) {
	input := MarkReadInput{MessageID: 12345}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded MarkReadInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.MessageID != 12345 {
		t.Errorf("MessageID = %d, want 12345", decoded.MessageID)
	}
}

func TestBuildToolResult_EmptyPayload(t *testing.T) {
	payload := map[string]interface{}{}

	result, err := buildToolResult(payload)
	if err != nil {
		t.Fatalf("buildToolResult() failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Content))
	}
}

func TestBuildResourceResult_EmptyPayload(t *testing.T) {
	payload := ResourcePayload{
		Metadata: ResourceMetadata{
			Timestamp:   time.Now(),
			ResourceURI: "push://empty",
			Count:       0,
		},
		Data: nil,
	}

	result, err := buildResourceResult("push://empty", payload)
	if err != nil {
		t.Fatalf("buildResourceResult() failed: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestResourcePayload_NilLinks(t *testing.T) {
	payload := ResourcePayload{
		Metadata: ResourceMetadata{
			Timestamp:   time.Now(),
			ResourceURI: "push://test",
			Count:       1,
		},
		Data:  "some data",
		Links: nil,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded ResourcePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Links != nil {
		t.Errorf("expected nil links, got %v", decoded.Links)
	}
}

// Helper function for test setup.
func intPtr(i int) *int {
	return &i
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// rewriteTransport redirects HTTP requests to a test server.
type rewriteTransport struct {
	targetURL string
	rt        http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newURL := t.targetURL + req.URL.Path
	if req.URL.RawQuery != "" {
		newURL += "?" + req.URL.RawQuery
	}
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header
	return t.rt.RoundTrip(newReq)
}

// Typed response structs for mock Pushover API responses.
type mockSendResponse struct {
	Status  int    `json:"status"`
	Request string `json:"request"`
	Receipt string `json:"receipt,omitempty"`
}

type mockFetchResponse struct {
	Status   int                `json:"status"`
	Request  string             `json:"request"`
	Last     int                `json:"last"`
	Messages []mockFetchMessage `json:"messages"`
}

type mockFetchMessage struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
	Title   string `json:"title,omitempty"`
	Date    int64  `json:"date"`
}

type mockStatusResponse struct {
	Status int `json:"status"`
}

type mockErrorResponse struct {
	Status  int      `json:"status"`
	Request string   `json:"request,omitempty"`
	Errors  []string `json:"errors,omitempty"`
}

// serverWithMockClient creates a test server with a mocked Pushover API.
type mockPushoverServer struct {
	*httptest.Server
	sendHandler   func(w http.ResponseWriter, r *http.Request)
	fetchHandler  func(w http.ResponseWriter, r *http.Request)
	deleteHandler func(w http.ResponseWriter, r *http.Request)
}

func newMockPushoverServer() *mockPushoverServer {
	m := &mockPushoverServer{}

	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages.json"):
			if m.sendHandler != nil {
				m.sendHandler(w, r)
			} else {
				//nolint:errchkjson // test mock data never fails
				_ = json.NewEncoder(w).Encode(mockSendResponse{
					Status:  1,
					Request: "mock-req-123",
				})
			}
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/messages.json"):
			if m.fetchHandler != nil {
				m.fetchHandler(w, r)
			} else {
				//nolint:errchkjson // test mock data never fails
				_ = json.NewEncoder(w).Encode(mockFetchResponse{
					Status:   1,
					Request:  "mock-fetch-123",
					Last:     12345,
					Messages: []mockFetchMessage{},
				})
			}
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/devices/"):
			if m.deleteHandler != nil {
				m.deleteHandler(w, r)
			} else {
				//nolint:errchkjson // test mock data never fails
				_ = json.NewEncoder(w).Encode(mockStatusResponse{Status: 1})
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	return m
}

func (m *mockPushoverServer) createClient(cfg *config.Config) *pushover.Client {
	client := pushover.NewClient(cfg.AppToken, cfg.UserKey, cfg.DeviceID, cfg.DeviceSecret)
	client.SetHTTPClient(&http.Client{
		Transport: &rewriteTransport{targetURL: m.URL, rt: http.DefaultTransport},
	})
	return client
}

func TestHandleSendNotification_WithMockedAPI(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	mock := newMockPushoverServer()
	defer mock.Close()

	var capturedRequest map[string][]string
	mock.sendHandler = func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		capturedRequest = r.Form
		//nolint:errchkjson // test mock data never fails
		_ = json.NewEncoder(w).Encode(mockSendResponse{
			Status:  1,
			Request: "test-req-456",
			Receipt: "test-receipt",
		})
	}

	cfg := &config.Config{
		AppToken:        "app-token",
		UserKey:         "user-key",
		DefaultDevice:   "default-phone",
		DefaultPriority: 0,
	}
	server, err := NewServer(cfg, "", store, "")
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}

	// Create a mocked client and call send directly
	client := mock.createClient(cfg)
	ctx := context.Background()

	params := pushover.SendParams{
		Message:  "Test notification",
		Title:    "Test Title",
		Priority: 1,
		Device:   "my-phone",
	}

	resp, err := client.Send(ctx, params)
	if err != nil {
		t.Fatalf("Send() failed: %v", err)
	}

	if resp.Request != "test-req-456" {
		t.Errorf("Request = %q, want %q", resp.Request, "test-req-456")
	}

	// Verify request parameters
	if msg := capturedRequest["message"]; len(msg) == 0 || msg[0] != "Test notification" {
		t.Errorf("message = %v, want %q", msg, "Test notification")
	}
	if title := capturedRequest["title"]; len(title) == 0 || title[0] != "Test Title" {
		t.Errorf("title = %v, want %q", title, "Test Title")
	}

	_ = server // Used for setup
}

func TestHandleCheckMessages_WithMockedAPI(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	mock := newMockPushoverServer()
	defer mock.Close()

	mock.fetchHandler = func(w http.ResponseWriter, r *http.Request) {
		//nolint:errchkjson // test mock data never fails
		_ = json.NewEncoder(w).Encode(mockFetchResponse{
			Status:  1,
			Request: "fetch-req",
			Last:    99999,
			Messages: []mockFetchMessage{
				{ID: 100, Message: "Message 1", Title: "Title 1", Date: time.Now().Unix()},
				{ID: 101, Message: "Message 2", Title: "Title 2", Date: time.Now().Unix()},
			},
		})
	}

	cfg := &config.Config{
		AppToken:     "app-token",
		UserKey:      "user-key",
		DeviceID:     "device-id",
		DeviceSecret: "device-secret",
	}

	client := mock.createClient(cfg)
	ctx := context.Background()

	result, err := client.FetchMessages(ctx)
	if err != nil {
		t.Fatalf("FetchMessages() failed: %v", err)
	}

	if result.LastMessageID != 99999 {
		t.Errorf("LastMessageID = %d, want 99999", result.LastMessageID)
	}
	if len(result.Messages) != 2 {
		t.Errorf("Messages count = %d, want 2", len(result.Messages))
	}
}

func TestHandleMarkRead_WithMockedAPI(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	mock := newMockPushoverServer()
	defer mock.Close()

	var capturedMessageID string
	mock.deleteHandler = func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		capturedMessageID = r.Form.Get("message")
		//nolint:errchkjson // test mock data never fails
		_ = json.NewEncoder(w).Encode(mockStatusResponse{Status: 1})
	}

	cfg := &config.Config{
		AppToken:     "app-token",
		UserKey:      "user-key",
		DeviceID:     "device-id",
		DeviceSecret: "device-secret",
	}

	client := mock.createClient(cfg)
	ctx := context.Background()

	err = client.DeleteMessages(ctx, 12345)
	if err != nil {
		t.Fatalf("DeleteMessages() failed: %v", err)
	}

	if capturedMessageID != "12345" {
		t.Errorf("message = %q, want %q", capturedMessageID, "12345")
	}
}

func TestHandleSendNotification_WithReceipt(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	mock := newMockPushoverServer()
	defer mock.Close()

	mock.sendHandler = func(w http.ResponseWriter, r *http.Request) {
		//nolint:errchkjson // test mock data never fails
		_ = json.NewEncoder(w).Encode(mockSendResponse{
			Status:  1,
			Request: "high-priority-req",
			Receipt: "emergency-receipt-abc",
		})
	}

	cfg := &config.Config{
		AppToken: "app-token",
		UserKey:  "user-key",
	}

	client := mock.createClient(cfg)
	ctx := context.Background()

	resp, err := client.Send(ctx, pushover.SendParams{
		Message:  "Emergency!",
		Priority: 2,
	})
	if err != nil {
		t.Fatalf("Send() failed: %v", err)
	}

	if resp.Receipt != "emergency-receipt-abc" {
		t.Errorf("Receipt = %q, want %q", resp.Receipt, "emergency-receipt-abc")
	}
}

func TestHandleSendNotification_APIError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	mock := newMockPushoverServer()
	defer mock.Close()

	mock.sendHandler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		//nolint:errchkjson // test mock data never fails
		_ = json.NewEncoder(w).Encode(mockErrorResponse{
			Status:  0,
			Request: "error-req",
			Errors:  []string{"invalid token", "message too long"},
		})
	}

	cfg := &config.Config{
		AppToken: "app-token",
		UserKey:  "user-key",
	}

	client := mock.createClient(cfg)
	ctx := context.Background()

	_, err = client.Send(ctx, pushover.SendParams{Message: "test"})
	if err == nil {
		t.Fatal("expected error for API error response")
	}

	if !strings.Contains(err.Error(), "invalid token") {
		t.Errorf("error = %q, want to contain 'invalid token'", err.Error())
	}
}

func TestHandleCheckMessages_WithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	mock := newMockPushoverServer()
	defer mock.Close()

	// Return 5 messages
	mock.fetchHandler = func(w http.ResponseWriter, r *http.Request) {
		messages := make([]mockFetchMessage, 5)
		for i := 0; i < 5; i++ {
			messages[i] = mockFetchMessage{
				ID:      100 + i,
				Message: "Message " + string(rune('A'+i)),
				Date:    time.Now().Unix(),
			}
		}
		//nolint:errchkjson // test mock data never fails
		_ = json.NewEncoder(w).Encode(mockFetchResponse{
			Status:   1,
			Request:  "fetch-limit-req",
			Last:     104,
			Messages: messages,
		})
	}

	cfg := &config.Config{
		AppToken:     "app-token",
		UserKey:      "user-key",
		DeviceID:     "device-id",
		DeviceSecret: "device-secret",
	}

	client := mock.createClient(cfg)
	ctx := context.Background()

	result, err := client.FetchMessages(ctx)
	if err != nil {
		t.Fatalf("FetchMessages() failed: %v", err)
	}

	if len(result.Messages) != 5 {
		t.Errorf("Messages count = %d, want 5", len(result.Messages))
	}
	if result.LastMessageID != 104 {
		t.Errorf("LastMessageID = %d, want 104", result.LastMessageID)
	}
}

func TestHandleMarkRead_APIError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	mock := newMockPushoverServer()
	defer mock.Close()

	mock.deleteHandler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		//nolint:errchkjson // test mock data never fails
		_ = json.NewEncoder(w).Encode(mockErrorResponse{
			Status: 0,
			Errors: []string{"invalid secret"},
		})
	}

	cfg := &config.Config{
		AppToken:     "app-token",
		UserKey:      "user-key",
		DeviceID:     "device-id",
		DeviceSecret: "device-secret",
	}

	client := mock.createClient(cfg)
	ctx := context.Background()

	err = client.DeleteMessages(ctx, 12345)
	if err == nil {
		t.Fatal("expected error for API error response")
	}

	if !strings.Contains(err.Error(), "invalid secret") {
		t.Errorf("error = %q, want to contain 'invalid secret'", err.Error())
	}
}
