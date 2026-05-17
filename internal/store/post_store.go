package store

import (
	"context"
	"errors"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrPostNotFound = errors.New("post not found")
)

type PostStore struct {
	master  DB
	replica DB
}

func NewPostStore(master, replica DB) *PostStore {
	return &PostStore{master: master, replica: replica}
}

func (s *PostStore) Create(ctx context.Context, post models.Post) (models.Post, error) {
	row := s.master.QueryRow(ctx,
		`INSERT INTO post (author_id, content) VALUES ($1, $2) RETURNING post_id, created_at`,
		post.AuthorID, post.Content,
	)
	if err := row.Scan(&post.PostID, &post.CreatedAt); err != nil {
		return models.Post{}, err
	}
	return post, nil
}

func (s *PostStore) GetByID(ctx context.Context, postID uuid.UUID) (models.Post, error) {
	var post models.Post
	err := s.replica.QueryRow(ctx,
		`SELECT post_id, author_id, content, created_at FROM post WHERE post_id=$1`,
		postID,
	).Scan(&post.PostID, &post.AuthorID, &post.Content, &post.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Post{}, ErrPostNotFound
		}
		return models.Post{}, err
	}
	return post, nil
}

func (s *PostStore) Update(ctx context.Context, postID, authorID uuid.UUID, content string) (models.Post, error) {
	var post models.Post
	err := s.master.QueryRow(ctx,
		`UPDATE post SET content=$1 WHERE post_id=$2 AND author_id=$3 RETURNING post_id, author_id, content, created_at`,
		content, postID, authorID,
	).Scan(&post.PostID, &post.AuthorID, &post.Content, &post.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Post{}, ErrPostNotFound
		}
		return models.Post{}, err
	}
	return post, nil
}

func (s *PostStore) Delete(ctx context.Context, postID, authorID uuid.UUID) error {
	tag, err := s.master.Exec(ctx,
		`DELETE FROM post WHERE post_id=$1 AND author_id=$2`,
		postID, authorID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPostNotFound
	}
	return nil
}

func (s *PostStore) Feed(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Post, error) {
	rows, err := s.replica.Query(ctx,
		`SELECT p.post_id, p.author_id, p.content, p.created_at
		 FROM post p
		 JOIN friendship f ON f.friend_id = p.author_id
		 WHERE f.user_id = $1
		 ORDER BY p.created_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(&p.PostID, &p.AuthorID, &p.Content, &p.CreatedAt); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}
// FeedFromAuthors returns posts from a specific set of authors, ordered by created_at DESC.
func (s *PostStore) FeedFromAuthors(ctx context.Context, authorIDs []uuid.UUID, limit, offset int) ([]models.Post, error) {
	if len(authorIDs) == 0 {
		return nil, nil
	}
	rows, err := s.replica.Query(ctx,
		`SELECT p.post_id, p.author_id, p.content, p.created_at
		 FROM post p
		 WHERE p.author_id = ANY($1)
		 ORDER BY p.created_at DESC
		 LIMIT $2 OFFSET $3`,
		authorIDs, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(&p.PostID, &p.AuthorID, &p.Content, &p.CreatedAt); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}
