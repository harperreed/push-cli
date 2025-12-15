// ABOUTME: Message conversion utilities between API and database formats.
// ABOUTME: Transforms Pushover API responses into database records.
package messages

import (
	"context"
	"strconv"
	"time"

	"github.com/harper/push/internal/db"
	"github.com/harper/push/internal/pushover"
)

// RecordsFromReceived converts API messages into database records.
func RecordsFromReceived(msgs []pushover.ReceivedMessage) []db.MessageRecord {
	records := make([]db.MessageRecord, 0, len(msgs))
	for _, msg := range msgs {
		received := time.Now()
		umid := msg.UMIDStr
		if umid == "" && msg.UMID != 0 {
			umid = strconv.FormatInt(msg.UMID, 10)
		}
		rec := db.MessageRecord{
			PushoverID: msg.PushoverID,
			UMID:       umid,
			Title:      msg.Title,
			Message:    msg.Message,
			App:        msg.App,
			AID:        msg.AID,
			Icon:       msg.Icon,
			ReceivedAt: received,
			Priority:   msg.Priority,
			URL:        msg.URL,
			Acked:      msg.Acked != 0,
			HTML:       msg.HTML != 0,
		}
		if msg.Date > 0 {
			sent := time.Unix(msg.Date, 0)
			rec.SentAt = &sent
		}
		records = append(records, rec)
	}
	return records
}

// PersistReceived converts and saves received messages, returning inserted count.
func PersistReceived(ctx context.Context, store *db.Store, msgs []pushover.ReceivedMessage) (int, error) {
	if len(msgs) == 0 {
		return 0, nil
	}
	records := RecordsFromReceived(msgs)
	return store.PersistMessages(ctx, records)
}
