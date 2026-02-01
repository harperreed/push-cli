// ABOUTME: History command for viewing persisted message history.
// ABOUTME: Queries local SQLite database with date and text filters.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/harper/push/internal/db"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	cmd.Flags().Bool("json", false, "output JSON (deprecated: use --format json)")
	cmd.Flags().String("format", "table", "output format: table, json, markdown, yaml")
	cmd.Flags().StringP("output", "o", "", "write output to file")
	cmd.Flags().Bool("include-sent", false, "include sent messages in output")

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
	format, _ := cmd.Flags().GetString("format")
	output, _ := cmd.Flags().GetString("output")
	includeSent, _ := cmd.Flags().GetBool("include-sent")

	// Handle deprecated --json flag
	if asJSON {
		format = "json"
	}

	if err := validateFormat(format); err != nil {
		return err
	}

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

	var sentRecords []db.SentRecord
	if includeSent {
		sentRecords, err = store.QuerySent(cmd.Context(), limit, since, search)
		if err != nil {
			return err
		}
	}

	// Generate output
	var result []byte
	switch strings.ToLower(format) {
	case "json":
		result, err = generateJSONOutput(records, sentRecords, includeSent)
	case "markdown":
		result = generateMarkdownOutput(records, sentRecords)
	case "yaml":
		result, err = generateYAMLOutput(records, sentRecords, since, search)
	default: // "table"
		if output != "" {
			return fmt.Errorf("table format does not support file output; use --format markdown, json, or yaml")
		}
		writeHistoryTable(cmd, records)
		if includeSent && len(sentRecords) > 0 {
			cmd.Println("\n--- Sent Messages ---")
			writeSentTable(cmd, sentRecords)
		}
		return nil
	}

	if err != nil {
		return err
	}

	// Write to file or stdout
	if output != "" {
		if err := writeToFile(output, result); err != nil {
			return err
		}
		cmd.Printf("Wrote history to %s\n", output)
		return nil
	}

	_, _ = cmd.OutOrStdout().Write(result)
	return nil
}

func validateFormat(format string) error {
	if format == "" {
		return nil
	}
	switch strings.ToLower(format) {
	case "table", "json", "markdown", "yaml":
		return nil
	default:
		return fmt.Errorf("invalid format %q; valid formats are: table, json, markdown, yaml", format)
	}
}

func generateJSONOutput(records []db.MessageRecord, sentRecords []db.SentRecord, _ bool) ([]byte, error) {
	var buf strings.Builder
	if err := writeHistoryJSONFull(&buf, records, sentRecords); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

func generateMarkdownOutput(records []db.MessageRecord, sentRecords []db.SentRecord) []byte {
	var buf strings.Builder
	writeHistoryMarkdown(&buf, records, sentRecords)
	return []byte(buf.String())
}

func generateYAMLOutput(records []db.MessageRecord, sentRecords []db.SentRecord, since *time.Time, search string) ([]byte, error) {
	var buf strings.Builder
	if err := writeHistoryYAML(&buf, records, sentRecords, since, search); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

func writeToFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil { //nolint:gosec // intentional file write
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func writeHistoryJSONFull(w io.Writer, records []db.MessageRecord, sentRecords []db.SentRecord) error {
	type exportData struct {
		ExportDate string             `json:"export_date"`
		Received   []db.MessageRecord `json:"received"`
		Sent       []db.SentRecord    `json:"sent,omitempty"`
	}

	data := exportData{
		ExportDate: time.Now().UTC().Format(time.RFC3339),
		Received:   records,
		Sent:       sentRecords,
	}

	if data.Received == nil {
		data.Received = []db.MessageRecord{}
	}
	if data.Sent == nil {
		data.Sent = []db.SentRecord{}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

//nolint:nestif // structure is clear and readable
func writeHistoryMarkdown(w io.Writer, records []db.MessageRecord, sentRecords []db.SentRecord) {
	_, _ = fmt.Fprintf(w, "# Push Message History\n\n")
	_, _ = fmt.Fprintf(w, "Export date: %s\n\n", time.Now().UTC().Format(time.RFC3339))

	_, _ = fmt.Fprintf(w, "## Received Messages\n\n")
	if len(records) == 0 {
		_, _ = fmt.Fprintf(w, "No messages found.\n\n")
	} else {
		for _, rec := range records {
			timestamp := rec.ReceivedAt.Local().Format("2006-01-02 15:04:05")
			if rec.Title != "" {
				_, _ = fmt.Fprintf(w, "### %s - %s\n", timestamp, rec.Title)
			} else {
				_, _ = fmt.Fprintf(w, "### %s\n", timestamp)
			}
			if rec.App != "" {
				_, _ = fmt.Fprintf(w, "**App**: %s\n", rec.App)
			}
			if rec.Priority != 0 {
				_, _ = fmt.Fprintf(w, "**Priority**: %d\n", rec.Priority)
			}
			_, _ = fmt.Fprintf(w, "**Message**: %s\n", rec.Message)
			if rec.URL != "" {
				_, _ = fmt.Fprintf(w, "**URL**: %s\n", rec.URL)
			}
			_, _ = fmt.Fprintf(w, "\n---\n\n")
		}
	}

	_, _ = fmt.Fprintf(w, "## Sent Messages\n\n")
	if len(sentRecords) == 0 {
		_, _ = fmt.Fprintf(w, "No messages found.\n\n")
	} else {
		for _, rec := range sentRecords {
			timestamp := rec.SentAt.Local().Format("2006-01-02 15:04:05")
			if rec.Title != "" {
				_, _ = fmt.Fprintf(w, "### %s - %s\n", timestamp, rec.Title)
			} else {
				_, _ = fmt.Fprintf(w, "### %s\n", timestamp)
			}
			if rec.Device != "" {
				_, _ = fmt.Fprintf(w, "**Device**: %s\n", rec.Device)
			}
			if rec.Priority != 0 {
				_, _ = fmt.Fprintf(w, "**Priority**: %d\n", rec.Priority)
			}
			_, _ = fmt.Fprintf(w, "**Message**: %s\n", rec.Message)
			_, _ = fmt.Fprintf(w, "\n---\n\n")
		}
	}
}

type yamlExport struct {
	Export   yamlExportMeta   `yaml:"export"`
	Received []yamlReceived   `yaml:"received"`
	Sent     []yamlSentRecord `yaml:"sent"`
}

type yamlExportMeta struct {
	Date    string      `yaml:"date"`
	Filters yamlFilters `yaml:"filters,omitempty"`
}

type yamlFilters struct {
	Since  string `yaml:"since,omitempty"`
	Search string `yaml:"search,omitempty"`
}

type yamlReceived struct {
	PushoverID int64  `yaml:"pushover_id"`
	ReceivedAt string `yaml:"received_at"`
	Title      string `yaml:"title,omitempty"`
	Message    string `yaml:"message"`
	App        string `yaml:"app,omitempty"`
	Priority   int    `yaml:"priority,omitempty"`
	URL        string `yaml:"url,omitempty"`
}

type yamlSentRecord struct {
	ID       int64  `yaml:"id"`
	SentAt   string `yaml:"sent_at"`
	Title    string `yaml:"title,omitempty"`
	Message  string `yaml:"message"`
	Device   string `yaml:"device,omitempty"`
	Priority int    `yaml:"priority,omitempty"`
}

func writeHistoryYAML(w io.Writer, records []db.MessageRecord, sentRecords []db.SentRecord, since *time.Time, search string) error {
	export := yamlExport{
		Export: yamlExportMeta{
			Date: time.Now().UTC().Format(time.RFC3339),
		},
		Received: make([]yamlReceived, 0, len(records)),
		Sent:     make([]yamlSentRecord, 0, len(sentRecords)),
	}

	if since != nil {
		export.Export.Filters.Since = since.Format("2006-01-02")
	}
	if search != "" {
		export.Export.Filters.Search = search
	}

	for _, rec := range records {
		export.Received = append(export.Received, yamlReceived{
			PushoverID: rec.PushoverID,
			ReceivedAt: rec.ReceivedAt.UTC().Format(time.RFC3339),
			Title:      rec.Title,
			Message:    rec.Message,
			App:        rec.App,
			Priority:   rec.Priority,
			URL:        rec.URL,
		})
	}

	for _, rec := range sentRecords {
		export.Sent = append(export.Sent, yamlSentRecord{
			ID:       rec.ID,
			SentAt:   rec.SentAt.UTC().Format(time.RFC3339),
			Title:    rec.Title,
			Message:  rec.Message,
			Device:   rec.Device,
			Priority: rec.Priority,
		})
	}

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(export)
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

func writeSentTable(cmd *cobra.Command, records []db.SentRecord) {
	if len(records) == 0 {
		cmd.Println("No sent messages found.")
		return
	}
	for _, rec := range records {
		timestamp := rec.SentAt.Local().Format(time.RFC3339)
		cmd.Printf("%s [%d] %s\n", timestamp, rec.ID, rec.Message)
		if rec.Title != "" {
			cmd.Printf("  Title: %s\n", rec.Title)
		}
		if rec.Device != "" {
			cmd.Printf("  Device: %s\n", rec.Device)
		}
		if rec.Priority != 0 {
			cmd.Printf("  Priority: %d\n", rec.Priority)
		}
	}
}
