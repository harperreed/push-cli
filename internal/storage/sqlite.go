// ABOUTME: SQLite storage backend for push data persistence.
// ABOUTME: Wraps the existing db.Store to implement the Storage interface.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SqliteStore provides SQLite-backed storage for push data.
type SqliteStore struct {
	db *sql.DB
}

// Compile-time check that SqliteStore implements Storage.
var _ Storage = (*SqliteStore)(nil)

// NewSqliteStore creates a new SQLite-backed store at the given path.
func NewSqliteStore(path string) (*SqliteStore, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if _, err := conn.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("configuring sqlite: %w", err)
	}

	store := &SqliteStore{db: conn}
	if err := store.migrate(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return store, nil
}

// Close releases the underlying SQL handle.
func (s *SqliteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SqliteStore) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS messages (
            id INTEGER PRIMARY KEY,
            pushover_id INTEGER UNIQUE,
            umid TEXT,
            title TEXT,
            message TEXT NOT NULL,
            app TEXT,
            aid INTEGER,
            icon TEXT,
            received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            sent_at DATETIME,
            priority INTEGER DEFAULT 0,
            url TEXT,
            acked INTEGER DEFAULT 0,
            html INTEGER DEFAULT 0
        );`,
		`CREATE TABLE IF NOT EXISTS sent (
            id INTEGER PRIMARY KEY,
            message TEXT NOT NULL,
            title TEXT,
            device TEXT,
            priority INTEGER DEFAULT 0,
            sent_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            request_id TEXT
        );`,
		`CREATE INDEX IF NOT EXISTS idx_messages_received_at ON messages(received_at);`,
		`CREATE INDEX IF NOT EXISTS idx_sent_sent_at ON sent(sent_at);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("running migration: %w", err)
		}
	}

	return nil
}

// PersistMessages inserts the provided message records, ignoring duplicates.
func (s *SqliteStore) PersistMessages(ctx context.Context, msgs []MessageRecord) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("database not initialized")
	}
	if len(msgs) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}

	inserted := 0
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO messages (
            pushover_id, umid, title, message, app, aid, icon,
            received_at, sent_at, priority, url, acked, html
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(pushover_id) DO UPDATE SET
            umid=excluded.umid,
            title=excluded.title,
            message=excluded.message,
            app=excluded.app,
            aid=excluded.aid,
            icon=excluded.icon,
            received_at=excluded.received_at,
            sent_at=excluded.sent_at,
            priority=excluded.priority,
            url=excluded.url,
            acked=excluded.acked,
            html=excluded.html;`)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, msg := range msgs {
		received := msg.ReceivedAt
		if received.IsZero() {
			received = time.Now()
		}
		var sent interface{}
		if msg.SentAt != nil {
			sent = msg.SentAt.UTC()
		} else {
			sent = nil
		}
		if _, err := stmt.ExecContext(ctx,
			msg.PushoverID,
			msg.UMID,
			msg.Title,
			msg.Message,
			msg.App,
			msg.AID,
			msg.Icon,
			received.UTC(),
			sent,
			msg.Priority,
			msg.URL,
			boolToInt(msg.Acked),
			boolToInt(msg.HTML),
		); err != nil {
			_ = tx.Rollback()
			return inserted, fmt.Errorf("insert message: %w", err)
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return inserted, fmt.Errorf("commit messages: %w", err)
	}

	return inserted, nil
}

// LogSent persists a sent notification entry.
func (s *SqliteStore) LogSent(ctx context.Context, rec SentRecord) error {
	if s == nil || s.db == nil {
		return errors.New("database not initialized")
	}

	sentAt := rec.SentAt
	if sentAt.IsZero() {
		sentAt = time.Now()
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sent (message, title, device, priority, sent_at, request_id) VALUES (?, ?, ?, ?, ?, ?);`,
		rec.Message,
		rec.Title,
		rec.Device,
		rec.Priority,
		sentAt.UTC(),
		rec.RequestID,
	)
	if err != nil {
		return fmt.Errorf("insert sent record: %w", err)
	}
	return nil
}

// QueryMessages returns persisted messages applying the optional filters.
func (s *SqliteStore) QueryMessages(ctx context.Context, limit int, since *time.Time, search string) ([]MessageRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not initialized")
	}
	if limit <= 0 {
		limit = 20
	}

	clauses := []string{"1=1"}
	args := []interface{}{}

	if since != nil && !since.IsZero() {
		clauses = append(clauses, "received_at >= ?")
		args = append(args, since.UTC())
	}

	if search != "" {
		like := fmt.Sprintf("%%%s%%", search)
		clauses = append(clauses, "(message LIKE ? OR title LIKE ?)")
		args = append(args, like, like)
	}

	query := fmt.Sprintf(`SELECT id, pushover_id, umid, title, message, app, aid, icon,
            received_at, sent_at, priority, url, acked, html
        FROM messages
        WHERE %s
        ORDER BY received_at DESC
        LIMIT ?;`, strings.Join(clauses, " AND "))
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []MessageRecord
	for rows.Next() {
		var rec MessageRecord
		var sent sql.NullTime
		var received time.Time
		var acked, html int
		if err := rows.Scan(
			&rec.ID,
			&rec.PushoverID,
			&rec.UMID,
			&rec.Title,
			&rec.Message,
			&rec.App,
			&rec.AID,
			&rec.Icon,
			&received,
			&sent,
			&rec.Priority,
			&rec.URL,
			&acked,
			&html,
		); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		rec.ReceivedAt = received
		if sent.Valid {
			val := sent.Time
			rec.SentAt = &val
		}
		rec.Acked = acked == 1
		rec.HTML = html == 1
		results = append(results, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history: %w", err)
	}

	return results, nil
}

// QuerySent returns persisted sent messages applying the optional filters.
func (s *SqliteStore) QuerySent(ctx context.Context, limit int, since *time.Time, search string) ([]SentRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not initialized")
	}
	if limit <= 0 {
		limit = 20
	}

	clauses := []string{"1=1"}
	args := []interface{}{}

	if since != nil && !since.IsZero() {
		clauses = append(clauses, "sent_at >= ?")
		args = append(args, since.UTC())
	}

	if search != "" {
		like := fmt.Sprintf("%%%s%%", search)
		clauses = append(clauses, "(message LIKE ? OR title LIKE ?)")
		args = append(args, like, like)
	}

	query := fmt.Sprintf(`SELECT id, message, title, device, priority, sent_at, request_id
        FROM sent
        WHERE %s
        ORDER BY sent_at DESC
        LIMIT ?;`, strings.Join(clauses, " AND "))
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sent: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []SentRecord
	for rows.Next() {
		var rec SentRecord
		var sentAt time.Time
		if err := rows.Scan(
			&rec.ID,
			&rec.Message,
			&rec.Title,
			&rec.Device,
			&rec.Priority,
			&sentAt,
			&rec.RequestID,
		); err != nil {
			return nil, fmt.Errorf("scan sent: %w", err)
		}
		rec.SentAt = sentAt
		results = append(results, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sent: %w", err)
	}

	return results, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
