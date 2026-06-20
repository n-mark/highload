package models

import (
	"time"

	"github.com/google/uuid"
)

type Post struct {
	PostID    uuid.UUID
	AuthorID  uuid.UUID
	Content   string
	CreatedAt time.Time
}

type CreatePostDTO struct {
	Content string `json:"content"`
}

type UpdatePostDTO struct {
	PostID  string `json:"post_id"`
	Content string `json:"content"`
}

type GetPostDTO struct {
	PostID    uuid.UUID `json:"post_id"`
	AuthorID  uuid.UUID `json:"author_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type FeedQueryDTO struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type FeedResponseDTO struct {
	Posts  []GetPostDTO `json:"posts"`
	Source string       `json:"source"`
}
