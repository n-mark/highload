package services

import (
	"context"
	"errors"
	"fmt"

	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
	"github.com/google/uuid"
)

var ErrEmptyMessage = errors.New("message text cannot be empty")

// DialogService handles business logic for the dialog subsystem.
type DialogService struct {
	dialogStore *store.DialogStore
}

func NewDialogService(dialogStore *store.DialogStore) *DialogService {
	return &DialogService{dialogStore: dialogStore}
}

// Send validates and persists a message from fromUserID to toUserID.
func (s *DialogService) Send(ctx context.Context, fromUserID, toUserID uuid.UUID, dto models.SendMessageDTO) (models.GetMessageDTO, error) {
	if dto.Text == "" {
		return models.GetMessageDTO{}, ErrEmptyMessage
	}

	if fromUserID == toUserID {
		return models.GetMessageDTO{}, fmt.Errorf("cannot send message to yourself")
	}

	msg, err := s.dialogStore.Send(ctx, fromUserID, toUserID, dto.Text)
	if err != nil {
		return models.GetMessageDTO{}, err
	}

	return models.GetMessageDTO{
		ID:         msg.ID,
		FromUserID: msg.FromUserID,
		ToUserID:   msg.ToUserID,
		Text:       msg.Text,
		CreatedAt:  msg.CreatedAt,
	}, nil
}

// List returns all messages in the conversation between userA and userB.
func (s *DialogService) List(ctx context.Context, userA, userB uuid.UUID) ([]models.GetMessageDTO, error) {
	msgs, err := s.dialogStore.List(ctx, userA, userB)
	if err != nil {
		return nil, err
	}

	dtos := make([]models.GetMessageDTO, 0, len(msgs))
	for _, m := range msgs {
		dtos = append(dtos, models.GetMessageDTO{
			ID:         m.ID,
			FromUserID: m.FromUserID,
			ToUserID:   m.ToUserID,
			Text:       m.Text,
			CreatedAt:  m.CreatedAt,
		})
	}
	return dtos, nil
}
