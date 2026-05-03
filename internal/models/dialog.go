package models

import (
	"time"

	"github.com/google/uuid"
)

// Message is a single chat message stored in Citus.
type Message struct {
	ID             uuid.UUID `json:"id"`
	ConversationID string    `json:"conversation_id"` // shard key: sorted pair of user UUIDs
	FromUserID     uuid.UUID `json:"from_user_id"`
	ToUserID       uuid.UUID `json:"to_user_id"`
	Text           string    `json:"text"`
	CreatedAt      time.Time `json:"created_at"`
}

// SendMessageDTO is the request body for POST /dialog/{user_id}/send.
type SendMessageDTO struct {
	Text string `json:"text"`
}

// GetMessageDTO is what we return to the client.
type GetMessageDTO struct {
	ID         uuid.UUID `json:"id"`
	FromUserID uuid.UUID `json:"from_user_id"`
	ToUserID   uuid.UUID `json:"to_user_id"`
	Text       string    `json:"text"`
	CreatedAt  time.Time `json:"created_at"`
}
