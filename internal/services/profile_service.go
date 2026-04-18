package services

import (
	"context"

	"example.com/highload/myproject/internal/metrics"
	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
)

type ProfileService struct {
	store ProfileStoreInterface
}

type ProfileStoreInterface interface {
	Create(ctx context.Context, profile models.Profile) (models.Profile, error)
	Update(ctx context.Context, profile models.Profile) (models.Profile, error)
	GetByID(ctx context.Context, id uuid.UUID) (models.Profile, error)
	List(ctx context.Context, query models.QueryDTO) ([]models.Profile, error)
}

func NewProfileService(store ProfileStoreInterface) *ProfileService {
	return &ProfileService{store: store}
}

func (s *ProfileService) GetOne(ctx context.Context, id uuid.UUID) (models.GetProfileDTO, error) {
	profile, err := s.store.GetByID(ctx, id)
	if err != nil {
		return models.GetProfileDTO{}, err
	}

	return mapProfile(profile), nil
}

func (s *ProfileService) List(ctx context.Context, query models.QueryDTO) ([]models.GetProfileDTO, error) {
	profiles, err := s.store.List(ctx, query)
	if err != nil {
		return nil, err
	}

	result := make([]models.GetProfileDTO, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, mapProfile(profile))
	}
	return result, nil
}

func (s *ProfileService) Create(ctx context.Context, ownerID uuid.UUID, dto models.ProfileDTO) (models.GetProfileDTO, error) {
	profile := models.Profile{
		Name:        dto.Name,
		Surname:     dto.Surname,
		DateOfBirth: dto.DateOfBirth,
		Gender:      dto.Gender,
		Interests:   dto.Interests,
		City:        dto.City,
		Bio:         dto.Bio,
		OwnerID:     ownerID,
	}

	created, err := s.store.Create(ctx, profile)
	if err != nil {
		return models.GetProfileDTO{}, err
	}

	metrics.ProfilesCreatedTotal.Inc()
	return mapProfile(created), nil
}

func (s *ProfileService) Update(ctx context.Context, ownerID uuid.UUID, profileID uuid.UUID, dto models.ProfileDTO) (models.GetProfileDTO, error) {
	profile := models.Profile{
		ID:          profileID,
		OwnerID:     ownerID,
		Name:        dto.Name,
		Surname:     dto.Surname,
		DateOfBirth: dto.DateOfBirth,
		Gender:      dto.Gender,
		Interests:   dto.Interests,
		City:        dto.City,
		Bio:         dto.Bio,
	}

	updated, err := s.store.Update(ctx, profile)
	if err != nil {
		return models.GetProfileDTO{}, err
	}

	return mapProfile(updated), nil
}

func mapProfile(profile models.Profile) models.GetProfileDTO {
	return models.GetProfileDTO{
		ProfileID: profile.ID,
		UserID: profile.OwnerID,
		ProfileDTO: models.ProfileDTO{
			Name:        profile.Name,
			Surname:     profile.Surname,
			DateOfBirth: profile.DateOfBirth,
			Gender:      profile.Gender,
			Interests:   profile.Interests,
			City:        profile.City,
			Bio:         profile.Bio,
		},
	}
}
