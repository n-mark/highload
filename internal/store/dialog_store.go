package store

import (
	"context"
	"errors"
	"fmt"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrMessageNotFound = errors.New("message not found")

// DialogStore persists messages in Citus (sharded by conversation_id).
// It uses a separate DB connection pool so that the existing master/replica
// logic for the main PostgreSQL cluster is completely untouched.
type DialogStore struct {
	db DB // points to the Citus coordinator
}

func NewDialogStore(db DB) *DialogStore {
	return &DialogStore{db: db}
}

// conversationID returns a stable, canonical shard key for a pair of users.
// We always put the lexicographically smaller UUID first so that the key is
// identical regardless of who initiated the query or the message.
//
// "Lady Gaga effect" mitigation: because the key encodes the *pair*, a single
// prolific user's messages are distributed across every shard (one per
// conversation partner) instead of being funnelled into a single hot shard.
func conversationID(a, b uuid.UUID) string {
	as, bs := a.String(), b.String()
	if as < bs {
		return fmt.Sprintf("%s:%s", as, bs)
	}
	return fmt.Sprintf("%s:%s", bs, as)
}

// Send inserts a new message into the Citus cluster.
func (s *DialogStore) Send(ctx context.Context, fromUserID, toUserID uuid.UUID, text string) (models.Message, error) {
	convID := conversationID(fromUserID, toUserID)

	var msg models.Message
	err := s.db.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, from_user_id, to_user_id, text)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, conversation_id, from_user_id, to_user_id, text, created_at`,
		convID, fromUserID, toUserID, text,
	).Scan(&msg.ID, &msg.ConversationID, &msg.FromUserID, &msg.ToUserID, &msg.Text, &msg.CreatedAt)
	if err != nil {
		return models.Message{}, fmt.Errorf("dialog store send: %w", err)
	}
	return msg, nil
}

// List returns all messages for the conversation between userA and userB,
// ordered from oldest to newest.
// Because both sides share the same conversation_id, Citus routes this query
// to exactly one shard — no cross-shard fan-out needed.
func (s *DialogStore) List(ctx context.Context, userA, userB uuid.UUID) ([]models.Message, error) {
	convID := conversationID(userA, userB)

	rows, err := s.db.Query(ctx,
		`SELECT id, conversation_id, from_user_id, to_user_id, text, created_at
		 FROM messages
		 WHERE conversation_id = $1
		 ORDER BY created_at ASC`,
		convID,
	)
	if err != nil {
		return nil, fmt.Errorf("dialog store list: %w", err)
	}
	defer rows.Close()

	var msgs []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.FromUserID, &m.ToUserID, &m.Text, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("dialog store scan: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dialog store rows: %w", err)
	}

	// Return an empty slice (not nil) so the JSON response is [] not null
	if msgs == nil {
		return []models.Message{}, nil
	}
	return msgs, nil
}

// ensure compile-time that pgx.ErrNoRows is importable (used in future extensions)
var _ = pgx.ErrNoRows
