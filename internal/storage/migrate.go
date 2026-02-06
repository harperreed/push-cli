// ABOUTME: Data migration between push storage backends.
// ABOUTME: Copies messages and sent records from source to destination storage.
package storage

import (
	"context"
	"fmt"
	"os"
)

// MigrateSummary holds counts of migrated entities.
type MigrateSummary struct {
	Messages int
	Sent     int
}

// MigrateData copies all data from src to dst storage.
// It reads all messages and sent records from the source and writes them to
// the destination. The destination should be empty before calling this function.
func MigrateData(ctx context.Context, src, dst Storage) (*MigrateSummary, error) {
	summary := &MigrateSummary{}

	// Migrate received messages (use a large limit to get all)
	messages, err := src.QueryMessages(ctx, 100000, nil, "")
	if err != nil {
		return nil, fmt.Errorf("query source messages: %w", err)
	}

	if len(messages) > 0 {
		inserted, err := dst.PersistMessages(ctx, messages)
		if err != nil {
			return nil, fmt.Errorf("persist messages to destination: %w", err)
		}
		summary.Messages = inserted
	}

	// Migrate sent records
	sentRecords, err := src.QuerySent(ctx, 100000, nil, "")
	if err != nil {
		return nil, fmt.Errorf("query source sent records: %w", err)
	}

	for _, rec := range sentRecords {
		if ctx.Err() != nil {
			return summary, ctx.Err()
		}
		if err := dst.LogSent(ctx, rec); err != nil {
			return nil, fmt.Errorf("log sent record to destination: %w", err)
		}
		summary.Sent++
	}

	return summary, nil
}

// IsDirNonEmpty checks whether a directory exists and contains any files or subdirectories.
// Returns false if the directory does not exist or is empty.
func IsDirNonEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read directory %q: %w", path, err)
	}
	// Ignore .lock files from mdstore
	count := 0
	for _, e := range entries {
		if e.Name() != ".lock" {
			count++
		}
	}
	return count > 0, nil
}
