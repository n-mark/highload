package models

import "github.com/google/uuid"

type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
	PasswordHash string
	Phone        string
}

type CreateUserDTO struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type GetUserDTO struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
}

type UpdateUserDTO struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
}
