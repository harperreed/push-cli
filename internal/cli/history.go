// ABOUTME: History command for viewing persisted message history.
// ABOUTME: Queries local SQLite database with date and text filters.
package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/araddon/dateparse"
	"github.com/harper/push/internal/db"
	"github.com/spf13/cobra"
)

func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show persisted message history",
		RunE:  runHistory,
	}

	cmd.Flags().IntP("limit", "n", 20, "limit number of rows")
	cmd.Flags().String("since", "", "filter by natural language date (e.g. yesterday)")
	cmd.Flags().String("search", "", "search text")
	cmd.Flags().Bool("json", false, "output JSON")

	return cmd
}

func runHistory(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	if limit <= 0 {
		limit = 20
	}

	sinceStr, _ := cmd.Flags().GetString("since")
	search, _ := cmd.Flags().GetString("search")
	asJSON, _ := cmd.Flags().GetBool("json")

	var since *time.Time
	if sinceStr != "" {
		parsed, err := dateparse.ParseLocal(sinceStr)
		if err != nil {
			return fmt.Errorf("parse --since: %w", err)
		}
		since = &parsed
	}

	store, _, err := openStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	records, err := store.QueryMessages(cmd.Context(), limit, since, search)
	if err != nil {
		return err
	}

	if asJSON {
		return writeHistoryJSON(cmd, records)
	}
	writeHistoryTable(cmd, records)
	return nil
}

func writeHistoryJSON(cmd *cobra.Command, records []db.MessageRecord) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(records)
}

func writeHistoryTable(cmd *cobra.Command, records []db.MessageRecord) {
	if len(records) == 0 {
		cmd.Println("No history found.")
		return
	}
	for _, rec := range records {
		timestamp := rec.ReceivedAt.Local().Format(time.RFC3339)
		cmd.Printf("%s [%d] %s\n", timestamp, rec.PushoverID, rec.Message)
		if rec.Title != "" {
			cmd.Printf("  Title: %s\n", rec.Title)
		}
		if rec.URL != "" {
			cmd.Printf("  URL: %s\n", rec.URL)
		}
		if rec.Priority != 0 {
			cmd.Printf("  Priority: %d\n", rec.Priority)
		}
		if rec.App != "" {
			cmd.Printf("  App: %s\n", rec.App)
		}
	}
}
