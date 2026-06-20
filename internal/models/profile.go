package models

import (
	"time"

	"github.com/google/uuid"
)

type Gender string

const (
	GenderMale   Gender = "MALE"
	GenderFemale Gender = "FEMALE"
)

type Profile struct {
	ID          uuid.UUID
	Name        string
	Surname     string
	DateOfBirth time.Time
	Gender      Gender
	Interests   string
	City        string
	Bio         string
	OwnerID     uuid.UUID
}

type ProfileDTO struct {
	Name        string    `json:"name"`
	Surname     string    `json:"surname"`
	DateOfBirth time.Time `json:"date_of_birth"`
	Gender      Gender    `json:"gender"`
	Interests   string    `json:"interests"`
	City        string    `json:"city"`
	Bio         string    `json:"bio"`
}

type GetProfileDTO struct {
	ProfileDTO
	ProfileID uuid.UUID `json:"profile_id"`
	UserID    uuid.UUID `json:"user_id"`
}

type QueryDTO struct {
	Query   string `json:"query"`
	Gender  Gender `json:"gender"`
	City    string `json:"city"`
	AgeFrom int    `json:"age_from"`
	AgeTo   int    `json:"age_to"`
	Count   int    `json:"count"`
}
