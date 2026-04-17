package services

import (
	"context"

	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
	"github.com/google/uuid"
)

type FriendService struct {
	store *store.FriendStore
}

func NewFriendService(store *store.FriendStore) *FriendService {
	return &FriendService{store: store}
}

func (s *FriendService) AddFriend(ctx context.Context, userID uuid.UUID, dto models.FriendActionDTO) error {
	friendID, err := uuid.Parse(dto.FriendID)
	if err != nil {
		return store.ErrFriendNotFound
	}
	return s.store.AddFriend(ctx, userID, friendID)
}

func (s *FriendService) DeleteFriend(ctx context.Context, userID uuid.UUID, dto models.FriendActionDTO) error {
	friendID, err := uuid.Parse(dto.FriendID)
	if err != nil {
		return store.ErrFriendNotFound
	}
	return s.store.DeleteFriend(ctx, userID, friendID)
}