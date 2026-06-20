package models

import "github.com/google/uuid"

type Friendship struct {
	UserID   uuid.UUID
	FriendID uuid.UUID
}

type FriendActionDTO struct {
	FriendID string `json:"friend_id"`
}
