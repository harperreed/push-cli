// ABOUTME: Markdown file-based storage backend for push data persistence.
// ABOUTME: Stores messages and sent records as individual markdown files with YAML frontmatter.
package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/harper/suite/mdstore"
	"gopkg.in/yaml.v3"
)

// MarkdownStore provides file-based storage for push data using markdown files.
type MarkdownStore struct {
	dataDir string
}

// Compile-time check that MarkdownStore implements Storage.
var _ Storage = (*MarkdownStore)(nil)

// NewMarkdownStore creates a new markdown-backed store rooted at dataDir.
func NewMarkdownStore(dataDir string) (*MarkdownStore, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data directory path is empty")
	}
	if err := mdstore.EnsureDir(dataDir); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	// Pre-create subdirectories
	for _, sub := range []string{"received", "sent"} {
		if err := mdstore.EnsureDir(filepath.Join(dataDir, sub)); err != nil {
			return nil, fmt.Errorf("create %s directory: %w", sub, err)
		}
	}
	return &MarkdownStore{dataDir: dataDir}, nil
}

// Close releases resources. For MarkdownStore this is a no-op.
func (s *MarkdownStore) Close() error {
	return nil
}

// messageFrontmatter holds the YAML frontmatter for a received message file.
type messageFrontmatter struct {
	PushoverID int64  `yaml:"pushover_id"`
	UMID       string `yaml:"umid,omitempty"`
	Title      string `yaml:"title,omitempty"`
	App        string `yaml:"app,omitempty"`
	AID        int64  `yaml:"aid,omitempty"`
	Icon       string `yaml:"icon,omitempty"`
	ReceivedAt string `yaml:"received_at"`
	SentAt     string `yaml:"sent_at,omitempty"`
	Priority   int    `yaml:"priority,omitempty"`
	URL        string `yaml:"url,omitempty"`
	Acked      bool   `yaml:"acked,omitempty"`
	HTML       bool   `yaml:"html,omitempty"`
}

// sentFrontmatter holds the YAML frontmatter for a sent message file.
type sentFrontmatter struct {
	Title     string `yaml:"title,omitempty"`
	Device    string `yaml:"device,omitempty"`
	Priority  int    `yaml:"priority,omitempty"`
	SentAt    string `yaml:"sent_at"`
	RequestID string `yaml:"request_id,omitempty"`
}

// PersistMessages inserts or updates message records as markdown files.
// Each message is stored as received/<pushover_id>.md with frontmatter + body.
func (s *MarkdownStore) PersistMessages(ctx context.Context, msgs []MessageRecord) (int, error) {
	if len(msgs) == 0 {
		return 0, nil
	}

	inserted := 0
	var writeErr error

	err := mdstore.WithLock(s.dataDir, func() error {
		for _, msg := range msgs {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			received := msg.ReceivedAt
			if received.IsZero() {
				received = time.Now()
			}

			fm := messageFrontmatter{
				PushoverID: msg.PushoverID,
				UMID:       msg.UMID,
				Title:      msg.Title,
				App:        msg.App,
				AID:        msg.AID,
				Icon:       msg.Icon,
				ReceivedAt: mdstore.FormatTime(received.UTC()),
				Priority:   msg.Priority,
				URL:        msg.URL,
				Acked:      msg.Acked,
				HTML:       msg.HTML,
			}
			if msg.SentAt != nil {
				fm.SentAt = mdstore.FormatTime(msg.SentAt.UTC())
			}

			content, err := mdstore.RenderFrontmatter(&fm, "\n"+msg.Message+"\n")
			if err != nil {
				writeErr = fmt.Errorf("render message %d: %w", msg.PushoverID, err)
				return writeErr
			}

			filename := fmt.Sprintf("%d.md", msg.PushoverID)
			path := filepath.Join(s.dataDir, "received", filename)
			if err := mdstore.AtomicWrite(path, []byte(content)); err != nil {
				writeErr = fmt.Errorf("write message %d: %w", msg.PushoverID, err)
				return writeErr
			}
			inserted++
		}
		return nil
	})

	if err != nil {
		return inserted, err
	}
	return inserted, writeErr
}

// LogSent persists a sent notification entry as a markdown file.
// Each sent record is stored as sent/<timestamp>-<slug>.md.
func (s *MarkdownStore) LogSent(ctx context.Context, rec SentRecord) error {
	sentAt := rec.SentAt
	if sentAt.IsZero() {
		sentAt = time.Now()
	}

	fm := sentFrontmatter{
		Title:     rec.Title,
		Device:    rec.Device,
		Priority:  rec.Priority,
		SentAt:    mdstore.FormatTime(sentAt.UTC()),
		RequestID: rec.RequestID,
	}

	content, err := mdstore.RenderFrontmatter(&fm, "\n"+rec.Message+"\n")
	if err != nil {
		return fmt.Errorf("render sent record: %w", err)
	}

	// Use timestamp + slug for filename uniqueness
	slug := mdstore.Slugify(rec.Message)
	if len(slug) > 40 {
		slug = slug[:40]
	}
	timestamp := sentAt.UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.md", timestamp, slug)

	return mdstore.WithLock(s.dataDir, func() error {
		path := filepath.Join(s.dataDir, "sent", filename)
		// Handle potential collision by appending request ID
		if _, err := os.Stat(path); err == nil && rec.RequestID != "" {
			filename = fmt.Sprintf("%s-%s-%s.md", timestamp, slug, rec.RequestID[:8])
			path = filepath.Join(s.dataDir, "sent", filename)
		}
		return mdstore.AtomicWrite(path, []byte(content))
	})
}

// QueryMessages returns persisted messages from markdown files.
// Uses simple string matching for search (no FTS5).
func (s *MarkdownStore) QueryMessages(ctx context.Context, limit int, since *time.Time, search string) ([]MessageRecord, error) {
	if limit <= 0 {
		limit = 20
	}

	dir := filepath.Join(s.dataDir, "received")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read received directory: %w", err)
	}

	var results []MessageRecord
	for _, entry := range entries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		rec, err := s.readMessageFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue // skip malformed files
		}

		// Apply since filter
		if since != nil && !since.IsZero() && rec.ReceivedAt.Before(*since) {
			continue
		}

		// Apply search filter (case-insensitive substring match)
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(rec.Message), searchLower) &&
				!strings.Contains(strings.ToLower(rec.Title), searchLower) {
				continue
			}
		}

		results = append(results, *rec)
	}

	// Sort by received_at descending (most recent first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ReceivedAt.After(results[j].ReceivedAt)
	})

	// Apply limit
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// QuerySent returns persisted sent messages from markdown files.
// Uses simple string matching for search (no FTS5).
func (s *MarkdownStore) QuerySent(ctx context.Context, limit int, since *time.Time, search string) ([]SentRecord, error) {
	if limit <= 0 {
		limit = 20
	}

	dir := filepath.Join(s.dataDir, "sent")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sent directory: %w", err)
	}

	var results []SentRecord
	for _, entry := range entries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		rec, err := s.readSentFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue // skip malformed files
		}

		// Apply since filter
		if since != nil && !since.IsZero() && rec.SentAt.Before(*since) {
			continue
		}

		// Apply search filter (case-insensitive substring match)
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(rec.Message), searchLower) &&
				!strings.Contains(strings.ToLower(rec.Title), searchLower) {
				continue
			}
		}

		results = append(results, *rec)
	}

	// Sort by sent_at descending (most recent first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].SentAt.After(results[j].SentAt)
	})

	// Apply limit
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// readMessageFile reads a received message markdown file and returns a MessageRecord.
func (s *MarkdownStore) readMessageFile(path string) (*MessageRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	yamlStr, body := mdstore.ParseFrontmatter(string(data))
	if yamlStr == "" {
		return nil, fmt.Errorf("no frontmatter in %s", path)
	}

	var fm messageFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, fmt.Errorf("parse frontmatter in %s: %w", path, err)
	}

	receivedAt, err := mdstore.ParseTime(fm.ReceivedAt)
	if err != nil {
		return nil, fmt.Errorf("parse received_at in %s: %w", path, err)
	}

	rec := &MessageRecord{
		PushoverID: fm.PushoverID,
		UMID:       fm.UMID,
		Title:      fm.Title,
		Message:    strings.TrimSpace(body),
		App:        fm.App,
		AID:        fm.AID,
		Icon:       fm.Icon,
		ReceivedAt: receivedAt,
		Priority:   fm.Priority,
		URL:        fm.URL,
		Acked:      fm.Acked,
		HTML:       fm.HTML,
	}

	if fm.SentAt != "" {
		sentAt, err := mdstore.ParseTime(fm.SentAt)
		if err == nil {
			rec.SentAt = &sentAt
		}
	}

	return rec, nil
}

// readSentFile reads a sent message markdown file and returns a SentRecord.
func (s *MarkdownStore) readSentFile(path string) (*SentRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	yamlStr, body := mdstore.ParseFrontmatter(string(data))
	if yamlStr == "" {
		return nil, fmt.Errorf("no frontmatter in %s", path)
	}

	var fm sentFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, fmt.Errorf("parse frontmatter in %s: %w", path, err)
	}

	sentAt, err := mdstore.ParseTime(fm.SentAt)
	if err != nil {
		return nil, fmt.Errorf("parse sent_at in %s: %w", path, err)
	}

	rec := &SentRecord{
		Message:   strings.TrimSpace(body),
		Title:     fm.Title,
		Device:    fm.Device,
		Priority:  fm.Priority,
		SentAt:    sentAt,
		RequestID: fm.RequestID,
	}

	return rec, nil
}
