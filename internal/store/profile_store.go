package store

import (
	"errors"
	"sync"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
)

var ErrProfileNotFound = errors.New("profile not found")

type ProfileStore struct {
	mu       sync.RWMutex
	profiles map[uuid.UUID]models.Profile
}

func NewProfileStore() *ProfileStore {
	return &ProfileStore{
		profiles: make(map[uuid.UUID]models.Profile),
	}
}

func (s *ProfileStore) Create(profile models.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.profiles[profile.ID] = profile
	return nil
}

func (s *ProfileStore) Update(profile models.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.profiles[profile.ID]; !ok {
		return ErrProfileNotFound
	}

	s.profiles[profile.ID] = profile
	return nil
}

func (s *ProfileStore) GetByID(id uuid.UUID) (models.Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, ok := s.profiles[id]
	if !ok {
		return models.Profile{}, ErrProfileNotFound
	}

	return profile, nil
}

func (s *ProfileStore) List(filter func(models.Profile) bool, limit int) []models.Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.Profile, 0)
	for _, profile := range s.profiles {
		if filter != nil && !filter(profile) {
			continue
		}
		result = append(result, profile)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}
