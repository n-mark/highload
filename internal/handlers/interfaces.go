package handlers

import (
	"context"
	"net/http"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
)

type UserService interface {
	CreateUser(ctx context.Context, dto models.CreateUserDTO) (models.GetUserDTO, error)
	UpdateUser(ctx context.Context, dto models.UpdateUserDTO) (models.GetUserDTO, error)
}

type ProfileService interface {
	GetOne(ctx context.Context, id uuid.UUID) (models.GetProfileDTO, error)
	List(ctx context.Context, query models.QueryDTO) ([]models.GetProfileDTO, error)
	Create(ctx context.Context, ownerID uuid.UUID, dto models.ProfileDTO) (models.GetProfileDTO, error)
	Update(ctx context.Context, ownerID uuid.UUID, profileID uuid.UUID, dto models.ProfileDTO) (models.GetProfileDTO, error)
}

type AuthService interface {
	Login(ctx context.Context, dto models.LoginDTO) (models.TokenDTO, error)
	Register(ctx context.Context, dto models.CreateUserDTO) (models.GetUserDTO, error)
}

type AuthMiddleware interface {
	RequireAuth(next http.Handler) http.Handler
}
