package services

import (
	"context"
	"errors"

	"example.com/highload/myproject/internal/auth"
	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type UserService struct {
	store  *store.UserStore
	hasher auth.PasswordHasher
}

func NewUserService(store *store.UserStore, hasher auth.PasswordHasher) *UserService {
	return &UserService{store: store, hasher: hasher}
}

func (s *UserService) CreateUser(ctx context.Context, dto models.CreateUserDTO) (models.GetUserDTO, error) {
	passwordHash, err := s.hasher.Hash(dto.Password)
	if err != nil {
		return models.GetUserDTO{}, err
	}

	user := models.User{
		Username:     dto.Username,
		Email:        dto.Email,
		PasswordHash: passwordHash,
	}

	created, err := s.store.Create(ctx, user)
	if err != nil {
		return models.GetUserDTO{}, err
	}

	return models.GetUserDTO{UserID: created.ID, Username: created.Username, Email: created.Email}, nil
}

func (s *UserService) UpdateUser(ctx context.Context, dto models.UpdateUserDTO) (models.GetUserDTO, error) {
	user, err := s.store.GetByID(ctx, dto.UserID)
	if err != nil {
		return models.GetUserDTO{}, err
	}

	user.Username = dto.Username
	user.Email = dto.Email

	updated, err := s.store.Update(ctx, user)
	if err != nil {
		return models.GetUserDTO{}, err
	}

	return models.GetUserDTO{UserID: updated.ID, Username: updated.Username, Email: updated.Email}, nil
}

func (s *UserService) ValidateCredentials(ctx context.Context, username, password string) (models.User, error) {
	user, err := s.store.GetByUsername(ctx, username)
	if err != nil {
		return models.User{}, ErrInvalidCredentials
	}

	if err := s.hasher.Compare(user.PasswordHash, password); err != nil {
		return models.User{}, ErrInvalidCredentials
	}

	return user, nil
}
