package handlers

import (
	"net/http"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
)

type UserService interface {
	CreateUser(dto models.CreateUserDTO) (models.GetUserDTO, error)
	UpdateUser(dto models.UpdateUserDTO) (models.GetUserDTO, error)
}

type ProfileService interface {
	GetOne(id uuid.UUID) (models.GetProfileDTO, error)
	List(query models.QueryDTO) []models.GetProfileDTO
	Create(ownerID uuid.UUID, dto models.ProfileDTO) (models.GetProfileDTO, error)
	Update(ownerID uuid.UUID, profileID uuid.UUID, dto models.ProfileDTO) (models.GetProfileDTO, error)
}

type AuthService interface {
	Login(dto models.LoginDTO) (models.TokenDTO, error)
	Register(dto models.CreateUserDTO) (models.GetUserDTO, error)
}

type AuthMiddleware interface {
	RequireAuth(next http.Handler) http.Handler
}
