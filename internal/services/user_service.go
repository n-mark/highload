package services

import (
	"errors"

	"example.com/highload/myproject/internal/auth"
	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
	"github.com/google/uuid"
)

var ErrInvalidCredentials = errors.New("invalid credentials")


type UserService struct {
	store  *store.UserStore
	hasher auth.PasswordHasher
}

func NewUserService(store *store.UserStore, hasher auth.PasswordHasher) *UserService {
	return &UserService{store: store, hasher: hasher}
}

func (s *UserService) CreateUser(dto models.CreateUserDTO) (models.GetUserDTO, error) {
	passwordHash, err := s.hasher.Hash(dto.Password)
	if err != nil {
		return models.GetUserDTO{}, err
	}

	user := models.User{
		ID:           uuid.New(),
		Username:     dto.Username,
		Email:        dto.Email,
		PasswordHash: passwordHash,
	}

	if err := s.store.Create(user); err != nil {
		return models.GetUserDTO{}, err
	}

	return models.GetUserDTO{UserID: user.ID, Username: user.Username, Email: user.Email}, nil
}

func (s *UserService) UpdateUser(dto models.UpdateUserDTO) (models.GetUserDTO, error) {
	user, err := s.store.GetByID(dto.UserID)
	if err != nil {
		return models.GetUserDTO{}, err
	}

	user.Username = dto.Username
	user.Email = dto.Email

	if err := s.store.Update(user); err != nil {
		return models.GetUserDTO{}, err
	}

	return models.GetUserDTO{UserID: user.ID, Username: user.Username, Email: user.Email}, nil
}

func (s *UserService) ValidateCredentials(username, password string) (models.User, error) {
	user, err := s.store.GetByUsername(username)
	if err != nil {
		return models.User{}, ErrInvalidCredentials
	}

	if err := s.hasher.Compare(user.PasswordHash, password); err != nil {
		return models.User{}, ErrInvalidCredentials
	}

	return user, nil
}
