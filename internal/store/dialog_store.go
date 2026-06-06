package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
	"github.com/tarantool/go-tarantool/v2"
)

var ErrMessageNotFound = errors.New("message not found")

// DialogStore persists messages in Tarantool via UDF (no direct space access).
type DialogStore struct {
	conn *tarantool.Connection
}

func NewDialogStore(conn *tarantool.Connection) *DialogStore {
	return &DialogStore{conn: conn}
}

// Send inserts a new message by calling the Lua stored procedure dialog_send.
func (s *DialogStore) Send(ctx context.Context, fromUserID, toUserID uuid.UUID, text string) (models.Message, error) {
	resp, err := s.conn.Call("dialog_send", []interface{}{
		[]interface{}{
			fromUserID.String(),
			toUserID.String(),
			text,
		},
	})
	if err != nil {
		return models.Message{}, fmt.Errorf("dialog store send: %w", err)
	}
	if len(resp) == 0 {
		return models.Message{}, errors.New("dialog store send: empty response")
	}
	return mapToMessage(resp[0])
}

// List returns all messages for the conversation between userA and userB.
func (s *DialogStore) List(ctx context.Context, userA, userB uuid.UUID) ([]models.Message, error) {
	resp, err := s.conn.Call("dialog_list", []interface{}{
		[]interface{}{
			userA.String(),
			userB.String(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dialog store list: %w", err)
	}
	if len(resp) == 0 {
		return []models.Message{}, nil
	}
	raw, ok := resp[0].([]interface{})
	if !ok {
		return nil, fmt.Errorf("dialog store list: unexpected response type %T", resp[0])
	}
	msgs := make([]models.Message, 0, len(raw))
	for _, item := range raw {
		m, err := mapToMessage(item)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// mapToMessage converts a Tarantool map response to models.Message.
func mapToMessage(v interface{}) (models.Message, error) {
	m := models.Message{}
	data, ok := v.(map[interface{}]interface{})
	if !ok {
		return m, fmt.Errorf("dialog store: unsupported response type %T", v)
	}
	if id, ok := data["id"].(string); ok {
		m.ID = uuid.MustParse(id)
	}
	if conv, ok := data["conversation_id"].(string); ok {
		m.ConversationID = conv
	}
	if from, ok := data["from_user_id"].(string); ok {
		m.FromUserID = uuid.MustParse(from)
	}
	if to, ok := data["to_user_id"].(string); ok {
		m.ToUserID = uuid.MustParse(to)
	}
	if text, ok := data["text"].(string); ok {
		m.Text = text
	}
	if ts, ok := data["created_at"].(float64); ok {
		m.CreatedAt = time.UnixMilli(int64(ts))
	} else if ts, ok := data["created_at"].(uint64); ok {
		m.CreatedAt = time.UnixMilli(int64(ts))
	}
	return m, nil
}
