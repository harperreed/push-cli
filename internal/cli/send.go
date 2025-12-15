// ABOUTME: Send command for dispatching push notifications.
// ABOUTME: Sends messages via Pushover Message API with logging.
package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/harper/push/internal/db"
	"github.com/harper/push/internal/pushover"
	"github.com/spf13/cobra"
)

func newSendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send [message]",
		Short: "Send a Pushover notification",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runSend,
	}

	cmd.Flags().StringP("title", "t", "", "notification title")
	cmd.Flags().IntP("priority", "p", 0, "priority (-2 to 2)")
	cmd.Flags().StringP("url", "u", "", "supplementary URL")
	cmd.Flags().String("url-title", "", "supplementary URL title")
	cmd.Flags().StringP("sound", "s", "", "notification sound")
	cmd.Flags().StringP("device", "d", "", "target device name")

	return cmd
}

func runSend(cmd *cobra.Command, args []string) error {
	cfg, _, err := loadConfig()
	if err != nil {
		return err
	}
	if err := cfg.ValidateSend(); err != nil {
		return err
	}

	message := strings.TrimSpace(strings.Join(args, " "))
	if message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	title, _ := cmd.Flags().GetString("title")
	priority, _ := cmd.Flags().GetInt("priority")
	if priority < -2 || priority > 2 {
		return fmt.Errorf("priority must be between -2 and 2")
	}
	urlVal, _ := cmd.Flags().GetString("url")
	urlTitle, _ := cmd.Flags().GetString("url-title")
	sound, _ := cmd.Flags().GetString("sound")
	device, _ := cmd.Flags().GetString("device")

	client := newClientFromConfig(cfg)
	ctx := cmd.Context()
	params := pushover.SendParams{
		Message:  message,
		Title:    title,
		Device:   device,
		Priority: priority,
		URL:      urlVal,
		URLTitle: urlTitle,
		Sound:    sound,
	}

	resp, err := client.Send(ctx, params)
	if err != nil {
		return err
	}

	if err := logSentMessage(ctx, message, title, device, priority, resp.Request); err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: unable to log sent message: %v\n", err)
	}

	cmd.Printf("✓ Notification sent. Request ID: %s\n", resp.Request)
	if resp.Receipt != "" {
		cmd.Printf("Receipt: %s\n", resp.Receipt)
	}
	return nil
}

func logSentMessage(ctx context.Context, message, title, device string, priority int, requestID string) error {
	store, _, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	rec := db.SentRecord{
		Message:   message,
		Title:     title,
		Device:    device,
		Priority:  priority,
		RequestID: requestID,
		SentAt:    time.Now(),
	}
	return store.LogSent(ctx, rec)
}
