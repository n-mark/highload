package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrProfileNotFound = errors.New("profile not found")

type ProfileStore struct {
	db *pgxpool.Pool
}

func NewProfileStore(db *pgxpool.Pool) *ProfileStore {
	return &ProfileStore{db: db}
}

func (s *ProfileStore) Create(ctx context.Context, profile models.Profile) (models.Profile, error) {
	row := s.db.QueryRow(ctx,
		`INSERT INTO profile (userid, name, surname, date_of_birth, gender, city, bio, interests)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING profile_id`,
		profile.OwnerID, profile.Name, profile.Surname, profile.DateOfBirth,
		string(profile.Gender), profile.City, profile.Bio, profile.Interests,
	)

	if err := row.Scan(&profile.ID); err != nil {
		return models.Profile{}, err
	}

	return profile, nil
}

func (s *ProfileStore) Update(ctx context.Context, profile models.Profile) (models.Profile, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE profile
		 SET name=$1, surname=$2, date_of_birth=$3, gender=$4, city=$5, bio=$6, interests=$7
		 WHERE profile_id=$8 AND userid=$9`,
		profile.Name, profile.Surname, profile.DateOfBirth,
		string(profile.Gender), profile.City, profile.Bio, profile.Interests,
		profile.ID, profile.OwnerID,
	)
	if err != nil {
		return models.Profile{}, err
	}
	if tag.RowsAffected() == 0 {
		return models.Profile{}, ErrProfileNotFound
	}

	return profile, nil
}

func (s *ProfileStore) GetByID(ctx context.Context, id uuid.UUID) (models.Profile, error) {
	var p models.Profile
	var gender string
	err := s.db.QueryRow(ctx,
		`SELECT profile_id, userid, name, surname, date_of_birth, gender, city, bio, interests
		 FROM profile WHERE profile_id=$1`,
		id,
	).Scan(&p.ID, &p.OwnerID, &p.Name, &p.Surname, &p.DateOfBirth, &gender, &p.City, &p.Bio, &p.Interests)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Profile{}, ErrProfileNotFound
		}
		return models.Profile{}, err
	}

	p.Gender = models.Gender(gender)
	return p, nil
}

func (s *ProfileStore) List(ctx context.Context, query models.QueryDTO) ([]models.Profile, error) {
	sql := `SELECT profile_id, userid, name, surname, date_of_birth, gender, city, bio, interests
	        FROM profile WHERE 1=1`
	args := []any{}
	argIdx := 1

	if query.Gender != "" {
		sql += fmt.Sprintf(" AND gender=$%d", argIdx)
		args = append(args, string(query.Gender))
		argIdx++
	}
	if query.City != "" {
		sql += fmt.Sprintf(" AND city=$%d", argIdx)
		args = append(args, query.City)
		argIdx++
	}
	if query.Query != "" {
		words := strings.Fields(query.Query)
		lettersOnly := regexp.MustCompile(`[^a-zA-Zа-яА-ЯёЁ]`)

		var tokens []string
		for _, w := range words {
			clean := lettersOnly.ReplaceAllString(w, "")
			if clean != "" {
				tokens = append(tokens, clean)
			}
		}

		for _, token := range tokens {
			sql += fmt.Sprintf(" AND (lower(name) LIKE $%d OR lower(surname) LIKE $%d)", argIdx, argIdx)
			args = append(args, strings.ToLower(token)+"%")
			argIdx++
		}
	}
	if query.AgeFrom > 0 {
		sql += fmt.Sprintf(" AND date_of_birth <= (now() - interval '%d years')", query.AgeFrom)
	}
	if query.AgeTo > 0 {
		sql += fmt.Sprintf(" AND date_of_birth >= (now() - interval '%d years')", query.AgeTo)
	}
	if query.Count > 0 {
		sql += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, query.Count)
	}

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []models.Profile
	for rows.Next() {
		var p models.Profile
		var gender string
		if err := rows.Scan(&p.ID, &p.OwnerID, &p.Name, &p.Surname, &p.DateOfBirth, &gender, &p.City, &p.Bio, &p.Interests); err != nil {
			return nil, err
		}
		p.Gender = models.Gender(gender)
		profiles = append(profiles, p)
	}

	return profiles, rows.Err()
}
