// ABOUTME: Messages command for fetching unread Pushover messages.
// ABOUTME: Polls the Open Client API and persists to local database.
package cli

import (
	"fmt"

	"github.com/harper/push/internal/messages"
	"github.com/harper/push/internal/pushover"
	"github.com/spf13/cobra"
)

func newMessagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Fetch unread messages from Pushover",
		RunE:  runMessages,
	}

	cmd.Flags().IntP("limit", "n", 10, "maximum messages to return")

	return cmd
}

func runMessages(cmd *cobra.Command, args []string) error {
	cfg, _, err := loadConfig()
	if err != nil {
		return err
	}
	if err := cfg.ValidateReceive(); err != nil {
		return err
	}

	limit, _ := cmd.Flags().GetInt("limit")
	if limit <= 0 {
		limit = 10
	}

	client := newClientFromConfig(cfg)
	ctx := cmd.Context()
	result, err := client.FetchMessages(ctx)
	if err != nil {
		return err
	}

	store, _, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	if _, err := messages.PersistReceived(ctx, store, result.Messages); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to persist messages: %v\n", err)
	}

	if last := highestMessageID(result, result.Messages); last > 0 {
		if err := client.DeleteMessages(ctx, last); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: unable to ack messages: %v\n", err)
		}
	}

	messages := result.Messages
	if len(messages) > limit {
		messages = messages[:limit]
	}

	if len(messages) == 0 {
		cmd.Println("No new messages.")
		return nil
	}

	for _, msg := range messages {
		cmd.Printf("[%d] %s\n", msg.PushoverID, msg.Message)
		if msg.Title != "" {
			cmd.Printf("  Title: %s\n", msg.Title)
		}
		if msg.App != "" {
			cmd.Printf("  App: %s\n", msg.App)
		}
		if msg.URL != "" {
			cmd.Printf("  URL: %s\n", msg.URL)
		}
		if msg.Priority != 0 {
			cmd.Printf("  Priority: %d\n", msg.Priority)
		}
	}

	return nil
}

func highestMessageID(result *pushover.FetchResult, msgs []pushover.ReceivedMessage) int64 {
	if result != nil && result.LastMessageID > 0 {
		return result.LastMessageID
	}
	var highest int64
	for _, msg := range msgs {
		if msg.PushoverID > highest {
			highest = msg.PushoverID
		}
	}
	return highest
}
