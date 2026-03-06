package services

import (
	"context"
	"time"

	"example.com/highload/myproject/internal/auth"
	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
)

const tokenTTL = 24 * time.Hour

type AuthService struct {
	users      *store.UserStore
	hasher     auth.PasswordHasher
	jwtManager *auth.JWTManager
}

func NewAuthService(users *store.UserStore, hasher auth.PasswordHasher, jwtManager *auth.JWTManager) *AuthService {
	return &AuthService{users: users, hasher: hasher, jwtManager: jwtManager}
}

func (s *AuthService) Login(ctx context.Context, dto models.LoginDTO) (models.TokenDTO, error) {
	userService := NewUserService(s.users, s.hasher)
	user, err := userService.ValidateCredentials(ctx, dto.Username, dto.Password)
	if err != nil {
		return models.TokenDTO{}, err
	}

	token, err := s.jwtManager.GenerateToken(user.ID.String(), tokenTTL)
	if err != nil {
		return models.TokenDTO{}, err
	}

	return models.TokenDTO{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(tokenTTL.Seconds()),
	}, nil
}

func (s *AuthService) Register(ctx context.Context, dto models.CreateUserDTO) (models.GetUserDTO, error) {
	userService := NewUserService(s.users, s.hasher)
	user, err := userService.CreateUser(ctx, dto)
	if err != nil {
		return models.GetUserDTO{}, err
	}

	return user, nil
}
