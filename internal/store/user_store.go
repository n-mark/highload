package store

import (
	"errors"
	"sync"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUserExists = errors.New("user already exists")

type UserStore struct {
	mu       sync.RWMutex
	users    map[uuid.UUID]models.User
	byName   map[string]uuid.UUID
	byEmail  map[string]uuid.UUID
}

func NewUserStore() *UserStore {
	return &UserStore{
		users:   make(map[uuid.UUID]models.User),
		byName:  make(map[string]uuid.UUID),
		byEmail: make(map[string]uuid.UUID),
	}
}

func (s *UserStore) Create(user models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byName[user.Username]; exists {
		return ErrUserExists
	}
	if _, exists := s.byEmail[user.Email]; exists {
		return ErrUserExists
	}

	s.users[user.ID] = user
	s.byName[user.Username] = user.ID
	s.byEmail[user.Email] = user.ID
	return nil
}

func (s *UserStore) Update(user models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[user.ID]; !exists {
		return ErrUserNotFound
	}

	if existingID, exists := s.byName[user.Username]; exists && existingID != user.ID {
		return ErrUserExists
	}
	if existingID, exists := s.byEmail[user.Email]; exists && existingID != user.ID {
		return ErrUserExists
	}

	s.users[user.ID] = user
	s.byName[user.Username] = user.ID
	s.byEmail[user.Email] = user.ID
	return nil
}

func (s *UserStore) GetByID(id uuid.UUID) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[id]
	if !ok {
		return models.User{}, ErrUserNotFound
	}
	return user, nil
}

func (s *UserStore) GetByUsername(username string) (models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byName[username]
	if !ok {
		return models.User{}, ErrUserNotFound
	}

	user, ok := s.users[id]
	if !ok {
		return models.User{}, ErrUserNotFound
	}

	return user, nil
}
