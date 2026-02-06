// ABOUTME: Storage interface for push data backends.
// ABOUTME: Defines the contract that SQLite and markdown implementations must satisfy.
package storage

import (
	"context"
	"time"
)

// MessageRecord mirrors the messages table schema.
type MessageRecord struct {
	ID         int64
	PushoverID int64
	UMID       string
	Title      string
	Message    string
	App        string
	AID        int64
	Icon       string
	ReceivedAt time.Time
	SentAt     *time.Time
	Priority   int
	URL        string
	Acked      bool
	HTML       bool
}

// SentRecord mirrors the sent table.
type SentRecord struct {
	ID        int64
	Message   string
	Title     string
	Device    string
	Priority  int
	SentAt    time.Time
	RequestID string
}

// Storage defines the interface for push data persistence.
// Implementations include SqliteStore and MarkdownStore.
type Storage interface {
	// PersistMessages inserts the provided message records, ignoring duplicates.
	// Returns the number of records inserted.
	PersistMessages(ctx context.Context, msgs []MessageRecord) (int, error)

	// LogSent persists a sent notification entry.
	LogSent(ctx context.Context, rec SentRecord) error

	// QueryMessages returns persisted messages applying the optional filters.
	QueryMessages(ctx context.Context, limit int, since *time.Time, search string) ([]MessageRecord, error)

	// QuerySent returns persisted sent messages applying the optional filters.
	QuerySent(ctx context.Context, limit int, since *time.Time, search string) ([]SentRecord, error)

	// Close releases resources held by the storage backend.
	Close() error
}
