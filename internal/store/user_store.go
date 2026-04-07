package store

import (
	"context"
	"errors"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUserExists = errors.New("user already exists")

type UserStore struct {
	master  DB
	replica DB
}

func NewUserStore(master, replica DB) *UserStore {
	return &UserStore{master: master, replica: replica}
}

func (s *UserStore) Create(ctx context.Context, user models.User) (models.User, error) {
	row := s.master.QueryRow(ctx,
		`INSERT INTO users (username, email, password, phone)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		user.Username, user.Email, user.PasswordHash, user.Phone,
	)

	if err := row.Scan(&user.ID); err != nil {
		return models.User{}, mapPgError(err, ErrUserExists)
	}

	return user, nil
}

func (s *UserStore) Update(ctx context.Context, user models.User) (models.User, error) {
	tag, err := s.master.Exec(ctx,
		`UPDATE users SET username=$1, email=$2, phone=$3 WHERE id=$4`,
		user.Username, user.Email, user.Phone, user.ID,
	)
	if err != nil {
		return models.User{}, mapPgError(err, ErrUserExists)
	}
	if tag.RowsAffected() == 0 {
		return models.User{}, ErrUserNotFound
	}

	return user, nil
}

func (s *UserStore) GetByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	var user models.User
	err := s.replica.QueryRow(ctx,
		`SELECT id, username, email, password, COALESCE(phone, '') FROM users WHERE id=$1`,
		id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrUserNotFound
		}
		return models.User{}, err
	}

	return user, nil
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (models.User, error) {
	var user models.User
	err := s.replica.QueryRow(ctx,
		`SELECT id, username, email, password, COALESCE(phone, '') FROM users WHERE username=$1`,
		username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrUserNotFound
		}
		return models.User{}, err
	}

	return user, nil
}
