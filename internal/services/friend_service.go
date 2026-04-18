package services

import (
	"context"
	"log/slog"
	"time"

	"example.com/highload/myproject/internal/feed"
	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
	"github.com/google/uuid"
)

type FriendService struct {
	store  *store.FriendStore
	worker *feed.Worker
}

func NewFriendService(store *store.FriendStore, worker *feed.Worker) *FriendService {
	return &FriendService{store: store, worker: worker}
}

func (s *FriendService) AddFriend(ctx context.Context, userID uuid.UUID, dto models.FriendActionDTO) error {
	friendID, err := uuid.Parse(dto.FriendID)
	if err != nil {
		return store.ErrFriendNotFound
	}

	// Insert into database
	dbStart := time.Now()
	if err := s.store.AddFriend(ctx, userID, friendID); err != nil {
		return err
	}
	slog.Debug("AddFriend: database insert completed",
		"duration", time.Since(dbStart),
		"userID", userID,
		"friendID", friendID,
	)

	// Publish event
	eventStart := time.Now()
	s.worker.Enqueue(feed.Event{
		Type:     feed.EventFriendAdded,
		FriendID: userID,
		AuthorID: friendID,
	})
	slog.Debug("AddFriend: event published",
		"duration", time.Since(eventStart),
		"userID", userID,
		"friendID", friendID,
	)

	return nil
}

func (s *FriendService) DeleteFriend(ctx context.Context, userID uuid.UUID, dto models.FriendActionDTO) error {
	friendID, err := uuid.Parse(dto.FriendID)
	if err != nil {
		return store.ErrFriendNotFound
	}

	if err := s.store.DeleteFriend(ctx, userID, friendID); err != nil {
		return err
	}

	// Invalidate cache so feed is rebuilt without removed friend's posts
	s.worker.Enqueue(feed.Event{
		Type:     feed.EventFriendRemoved,
		FriendID: userID,
		AuthorID: friendID,
	})

	return nil
}
