package services

import (
	"context"

	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
	"github.com/google/uuid"
)

type PostService struct {
	store *store.PostStore
}

func NewPostService(store *store.PostStore) *PostService {
	return &PostService{store: store}
}

func (s *PostService) CreatePost(ctx context.Context, authorID uuid.UUID, dto models.CreatePostDTO) (models.GetPostDTO, error) {
	if dto.Content == "" {
		return models.GetPostDTO{}, store.ErrPostNotFound
	}

	post, err := s.store.Create(ctx, models.Post{
		AuthorID: authorID,
		Content:  dto.Content,
	})
	if err != nil {
		return models.GetPostDTO{}, err
	}

	return models.GetPostDTO{
		PostID:    post.PostID,
		AuthorID:  post.AuthorID,
		Content:   post.Content,
		CreatedAt: post.CreatedAt,
	}, nil
}

func (s *PostService) GetPost(ctx context.Context, postID string) (models.GetPostDTO, error) {
	id, err := uuid.Parse(postID)
	if err != nil {
		return models.GetPostDTO{}, store.ErrPostNotFound
	}

	post, err := s.store.GetByID(ctx, id)
	if err != nil {
		return models.GetPostDTO{}, err
	}

	return models.GetPostDTO{
		PostID:    post.PostID,
		AuthorID:  post.AuthorID,
		Content:   post.Content,
		CreatedAt: post.CreatedAt,
	}, nil
}

func (s *PostService) UpdatePost(ctx context.Context, authorID uuid.UUID, dto models.UpdatePostDTO) (models.GetPostDTO, error) {
	if dto.Content == "" {
		return models.GetPostDTO{}, store.ErrPostNotFound
	}

	postID, err := uuid.Parse(dto.PostID)
	if err != nil {
		return models.GetPostDTO{}, store.ErrPostNotFound
	}

	post, err := s.store.Update(ctx, postID, authorID, dto.Content)
	if err != nil {
		return models.GetPostDTO{}, err
	}

	return models.GetPostDTO{
		PostID:    post.PostID,
		AuthorID:  post.AuthorID,
		Content:   post.Content,
		CreatedAt: post.CreatedAt,
	}, nil
}

func (s *PostService) DeletePost(ctx context.Context, authorID uuid.UUID, postID string) error {
	id, err := uuid.Parse(postID)
	if err != nil {
		return store.ErrPostNotFound
	}
	return s.store.Delete(ctx, id, authorID)
}

func (s *PostService) Feed(ctx context.Context, userID uuid.UUID, query models.FeedQueryDTO) ([]models.GetPostDTO, error) {
	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	posts, err := s.store.Feed(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	result := make([]models.GetPostDTO, 0, len(posts))
	for _, p := range posts {
		result = append(result, models.GetPostDTO{
			PostID:    p.PostID,
			AuthorID:  p.AuthorID,
			Content:   p.Content,
			CreatedAt: p.CreatedAt,
		})
	}
	return result, nil
}