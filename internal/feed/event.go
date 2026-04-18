package feed

import (
	"time"

	"github.com/google/uuid"
)

type EventType int

const (
	EventPostCreated EventType = iota
	EventPostUpdated
	EventPostDeleted
	EventFriendAdded
	EventFriendRemoved
	EventRebuildFeed
)

type Event struct {
	Type      EventType `json:"type"`
	PostID    uuid.UUID `json:"post_id"`
	AuthorID  uuid.UUID `json:"author_id"`
	FriendID  uuid.UUID `json:"friend_id"` // for friend add/remove events, FriendID is the other user
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
