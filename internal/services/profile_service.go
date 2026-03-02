package services

import (
	"strings"
	"time"

	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
	"github.com/google/uuid"
)

type ProfileService struct {
	store *store.ProfileStore
}

func NewProfileService(store *store.ProfileStore) *ProfileService {
	return &ProfileService{store: store}
}

func (s *ProfileService) GetOne(id uuid.UUID) (models.GetProfileDTO, error) {
	profile, err := s.store.GetByID(id)
	if err != nil {
		return models.GetProfileDTO{}, err
	}

	return mapProfile(profile), nil
}

func (s *ProfileService) List(query models.QueryDTO) []models.GetProfileDTO {
	profiles := s.store.List(func(profile models.Profile) bool {
		if query.Gender != "" && profile.Gender != query.Gender {
			return false
		}
		if query.City != "" && profile.City != query.City {
			return false
		}
		if query.Query != "" {
			if !containsIgnoreCase(profile.Name, query.Query) && !containsIgnoreCase(profile.Surname, query.Query) {
				return false
			}
		}
		age := calcAge(profile.DateOfBirth)
		if query.AgeFrom > 0 && age < query.AgeFrom {
			return false
		}
		if query.AgeTo > 0 && age > query.AgeTo {
			return false
		}
		return true
	}, query.Count)

	result := make([]models.GetProfileDTO, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, mapProfile(profile))
	}
	return result
}

func (s *ProfileService) Create(ownerID uuid.UUID, dto models.ProfileDTO) (models.GetProfileDTO, error) {
	profile := models.Profile{
		ID:          uuid.New(),
		Name:        dto.Name,
		Surname:     dto.Surname,
		DateOfBirth: dto.DateOfBirth,
		Gender:      dto.Gender,
		Interests:   dto.Interests,
		City:        dto.City,
		OwnerID:     ownerID,
	}

	if err := s.store.Create(profile); err != nil {
		return models.GetProfileDTO{}, err
	}

	return mapProfile(profile), nil
}

func (s *ProfileService) Update(ownerID uuid.UUID, profileID uuid.UUID, dto models.ProfileDTO) (models.GetProfileDTO, error) {
	profile, err := s.store.GetByID(profileID)
	if err != nil {
		return models.GetProfileDTO{}, err
	}

	if profile.OwnerID != ownerID {
		return models.GetProfileDTO{}, store.ErrProfileNotFound
	}

	profile.Name = dto.Name
	profile.Surname = dto.Surname
	profile.DateOfBirth = dto.DateOfBirth
	profile.Gender = dto.Gender
	profile.Interests = dto.Interests
	profile.City = dto.City

	if err := s.store.Update(profile); err != nil {
		return models.GetProfileDTO{}, err
	}

	return mapProfile(profile), nil
}

func mapProfile(profile models.Profile) models.GetProfileDTO {
	return models.GetProfileDTO{
		ProfileID: profile.ID,
		ProfileDTO: models.ProfileDTO{
			Name:        profile.Name,
			Surname:     profile.Surname,
			DateOfBirth: profile.DateOfBirth,
			Gender:      profile.Gender,
			Interests:   profile.Interests,
			City:        profile.City,
		},
	}
}

func calcAge(birthDate time.Time) int {
	now := time.Now()
	age := now.Year() - birthDate.Year()
	if now.YearDay() < birthDate.YearDay() {
		age--
	}
	return age
}

func containsIgnoreCase(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
