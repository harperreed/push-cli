// ABOUTME: MCP tool definitions and handlers.
// ABOUTME: Implements send, receive, history, and mark-read operations.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/harper/push/internal/db"
	"github.com/harper/push/internal/messages"
	"github.com/harper/push/internal/pushover"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerTools() {
	s.registerSendNotificationTool()
	s.registerCheckMessagesTool()
	s.registerListHistoryTool()
	s.registerMarkReadTool()
}

func (s *Server) registerSendNotificationTool() {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "Body of the notification",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Optional title",
			},
			"priority": map[string]any{
				"type":        "integer",
				"minimum":     -2,
				"maximum":     2,
				"description": "Priority from -2 (lowest) to 2 (highest). Defaults to config value.",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "Supplementary URL",
			},
			"sound": map[string]any{
				"type":        "string",
				"description": "Notification sound",
			},
			"device": map[string]any{
				"type":        "string",
				"description": "Target device name. Defaults to config's default_device.",
			},
		},
		"required": []string{"message"},
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "send_notification",
		Description: "Send a push notification through Pushover, mirroring the CLI 'send' command.",
		InputSchema: schema,
	}, s.handleSendNotification)
}

func (s *Server) registerCheckMessagesTool() {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Maximum number of messages to return in the response. Defaults to 10.",
			},
		},
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "check_messages",
		Description: "Poll the Pushover Open Client API, persist new messages, and return the newest ones.",
		InputSchema: schema,
	}, s.handleCheckMessages)
}

func (s *Server) registerListHistoryTool() {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Number of rows to return (default 20).",
			},
			"since": map[string]any{
				"type":        "string",
				"description": "Natural language or ISO date filter (e.g. 'yesterday', '2025-01-01').",
			},
			"search": map[string]any{
				"type":        "string",
				"description": "Full text search over message and title fields.",
			},
		},
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_history",
		Description: "Query persisted message history from the local SQLite database.",
		InputSchema: schema,
	}, s.handleListHistory)
}

func (s *Server) registerMarkReadTool() {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message_id": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Highest Pushover message ID to acknowledge/delete.",
			},
		},
		"required": []string{"message_id"},
	}

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "mark_read",
		Description: "Delete unread messages from Pushover up to (and including) the provided ID.",
		InputSchema: schema,
	}, s.handleMarkRead)
}

type SendNotificationInput struct {
	Message  string `json:"message"`
	Title    string `json:"title,omitempty"`
	Priority *int   `json:"priority,omitempty"`
	URL      string `json:"url,omitempty"`
	Sound    string `json:"sound,omitempty"`
	Device   string `json:"device,omitempty"`
}

type SendNotificationOutput struct {
	Message   string `json:"message"`
	Title     string `json:"title,omitempty"`
	Device    string `json:"device,omitempty"`
	Priority  int    `json:"priority"`
	RequestID string `json:"request_id"`
	Receipt   string `json:"receipt,omitempty"`
	Logged    bool   `json:"logged"`
	Warning   string `json:"warning,omitempty"`
}

func (s *Server) handleSendNotification(ctx context.Context, _ *mcp.CallToolRequest, input SendNotificationInput) (*mcp.CallToolResult, SendNotificationOutput, error) {
	if err := s.cfg.ValidateSend(); err != nil {
		return nil, SendNotificationOutput{}, err
	}
	if strings.TrimSpace(input.Message) == "" {
		return nil, SendNotificationOutput{}, fmt.Errorf("message is required")
	}

	priority := s.cfg.DefaultPriority
	if input.Priority != nil {
		priority = *input.Priority
	}
	if priority < -2 || priority > 2 {
		return nil, SendNotificationOutput{}, fmt.Errorf("priority must be between -2 and 2")
	}

	device := input.Device
	if device == "" {
		device = s.cfg.DefaultDevice
	}

	params := pushover.SendParams{
		Message:  input.Message,
		Title:    input.Title,
		Device:   device,
		Priority: priority,
		URL:      input.URL,
		Sound:    input.Sound,
	}

	client := s.newClient()
	resp, err := client.Send(ctx, params)
	if err != nil {
		return nil, SendNotificationOutput{}, err
	}

	output := SendNotificationOutput{
		Message:   input.Message,
		Title:     input.Title,
		Device:    device,
		Priority:  priority,
		RequestID: resp.Request,
		Receipt:   resp.Receipt,
	}

	record := db.SentRecord{
		Message:   input.Message,
		Title:     input.Title,
		Device:    device,
		Priority:  priority,
		SentAt:    time.Now(),
		RequestID: resp.Request,
	}
	if err := s.store.LogSent(ctx, record); err != nil {
		output.Warning = fmt.Sprintf("failed to log history: %v", err)
	} else {
		output.Logged = true
	}

	result, err := buildToolResult(output)
	if err != nil {
		return nil, output, err
	}
	return result, output, nil
}

type CheckMessagesInput struct {
	Limit *int `json:"limit,omitempty"`
}

type CheckMessagesOutput struct {
	Count      int                        `json:"count"`
	Returned   int                        `json:"returned"`
	Limit      int                        `json:"limit"`
	Persisted  int                        `json:"persisted"`
	AckedUpTo  int64                      `json:"acked_up_to,omitempty"`
	Messages   []pushover.ReceivedMessage `json:"messages"`
	Warning    string                     `json:"warning,omitempty"`
	AckWarning string                     `json:"ack_warning,omitempty"`
}

func (s *Server) handleCheckMessages(ctx context.Context, _ *mcp.CallToolRequest, input CheckMessagesInput) (*mcp.CallToolResult, CheckMessagesOutput, error) {
	if err := s.cfg.ValidateReceive(); err != nil {
		return nil, CheckMessagesOutput{}, err
	}

	limit := 10
	if input.Limit != nil && *input.Limit > 0 {
		limit = *input.Limit
	}

	client := s.newClient()
	result, err := client.FetchMessages(ctx)
	if err != nil {
		return nil, CheckMessagesOutput{}, err
	}

	persisted, persistErr := messages.PersistReceived(ctx, s.store, result.Messages)
	warning := ""
	if persistErr != nil {
		warning = persistErr.Error()
	}

	ackedID := determineAckID(result)
	ackWarning := ""
	if ackedID > 0 {
		if err := client.DeleteMessages(ctx, ackedID); err != nil {
			ackWarning = err.Error()
		}
	}

	outgoing := result.Messages
	if len(outgoing) > limit {
		outgoing = outgoing[:limit]
	}

	output := CheckMessagesOutput{
		Count:      len(result.Messages),
		Returned:   len(outgoing),
		Limit:      limit,
		Persisted:  persisted,
		AckedUpTo:  ackedID,
		Messages:   outgoing,
		Warning:    warning,
		AckWarning: ackWarning,
	}

	resultPayload, err := buildToolResult(output)
	if err != nil {
		return nil, output, err
	}
	return resultPayload, output, nil
}

type ListHistoryInput struct {
	Limit  *int    `json:"limit,omitempty"`
	Since  *string `json:"since,omitempty"`
	Search *string `json:"search,omitempty"`
}

type ListHistoryOutput struct {
	Count    int                `json:"count"`
	Limit    int                `json:"limit"`
	Since    *time.Time         `json:"since,omitempty"`
	Search   string             `json:"search,omitempty"`
	Messages []db.MessageRecord `json:"messages"`
}

func (s *Server) handleListHistory(ctx context.Context, _ *mcp.CallToolRequest, input ListHistoryInput) (*mcp.CallToolResult, ListHistoryOutput, error) {
	limit := 20
	if input.Limit != nil && *input.Limit > 0 {
		limit = *input.Limit
	}

	var sinceTime *time.Time
	if input.Since != nil && *input.Since != "" {
		parsed, err := dateparse.ParseLocal(*input.Since)
		if err != nil {
			return nil, ListHistoryOutput{}, fmt.Errorf("invalid since value: %w", err)
		}
		sinceTime = &parsed
	}

	searchVal := ""
	if input.Search != nil {
		searchVal = *input.Search
	}

	records, err := s.store.QueryMessages(ctx, limit, sinceTime, searchVal)
	if err != nil {
		return nil, ListHistoryOutput{}, err
	}

	output := ListHistoryOutput{
		Count:    len(records),
		Limit:    limit,
		Since:    sinceTime,
		Search:   searchVal,
		Messages: records,
	}

	result, err := buildToolResult(output)
	if err != nil {
		return nil, output, err
	}
	return result, output, nil
}

type MarkReadInput struct {
	MessageID int64 `json:"message_id"`
}

type MarkReadOutput struct {
	MessageID int64  `json:"message_id"`
	Status    string `json:"status"`
}

func (s *Server) handleMarkRead(ctx context.Context, _ *mcp.CallToolRequest, input MarkReadInput) (*mcp.CallToolResult, MarkReadOutput, error) {
	if err := s.cfg.ValidateReceive(); err != nil {
		return nil, MarkReadOutput{}, err
	}
	if input.MessageID <= 0 {
		return nil, MarkReadOutput{}, fmt.Errorf("message_id must be positive")
	}

	client := s.newClient()
	if err := client.DeleteMessages(ctx, input.MessageID); err != nil {
		return nil, MarkReadOutput{}, err
	}

	output := MarkReadOutput{MessageID: input.MessageID, Status: "acknowledged"}
	result, err := buildToolResult(output)
	if err != nil {
		return nil, output, err
	}
	return result, output, nil
}

func determineAckID(result *pushover.FetchResult) int64 {
	if result == nil {
		return 0
	}
	if result.LastMessageID > 0 {
		return result.LastMessageID
	}
	var highest int64
	for _, msg := range result.Messages {
		if msg.PushoverID > highest {
			highest = msg.PushoverID
		}
	}
	return highest
}

func buildToolResult(payload any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}
