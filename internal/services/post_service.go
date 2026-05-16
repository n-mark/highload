package services

import (
	"context"
	"log/slog"
	"time"

	"example.com/highload/myproject/internal/feed"
	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
	"github.com/google/uuid"
)

type PostService struct {
	store       *store.PostStore
	friendStore *store.FriendStore
	cache       *feed.Cache
	worker      *feed.Worker
}

func NewPostService(store *store.PostStore, friendStore *store.FriendStore, cache *feed.Cache, worker *feed.Worker) *PostService {
	return &PostService{store: store, friendStore: friendStore, cache: cache, worker: worker}
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

	result := models.GetPostDTO{
		PostID:    post.PostID,
		AuthorID:  post.AuthorID,
		Content:   post.Content,
		CreatedAt: post.CreatedAt,
	}

	// Pre-warm cache: immediately insert post into friends' feeds
	go s.prewarmCacheForFollowers(post.PostID, post.AuthorID, post.Content, post.CreatedAt)

	return result, nil
}

// prewarmCacheForFollowers synchronously inserts the post into the cache for all followers
func (s *PostService) prewarmCacheForFollowers(postID, authorID uuid.UUID, content string, createdAt time.Time) {
	ctx := context.Background()

	// Get followers from friend store (using replica for read)
	followerIDs, err := s.friendStore.GetFollowers(ctx, authorID)
	if err != nil {
		slog.Error("prewarm cache: failed to get friends", "authorID", authorID, "error", err)
		return
	}

	// Include the author themselves
	allIDs := make([]uuid.UUID, 0, len(followerIDs)+1)
	allIDs = append(allIDs, authorID) // add self first
	allIDs = append(allIDs, followerIDs...)

	if len(allIDs) == 0 {
		return
	}

	postDTO := models.GetPostDTO{
		PostID:    postID,
		AuthorID:  authorID,
		Content:   content,
		CreatedAt: createdAt,
	}

	s.cache.InsertPost(allIDs, postDTO)
	slog.Debug("prewarm cache: inserted post for", "followers", len(allIDs))
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

	result := models.GetPostDTO{
		PostID:    post.PostID,
		AuthorID:  post.AuthorID,
		Content:   post.Content,
		CreatedAt: post.CreatedAt,
	}

	// Immediately update post in cache (much faster than full rebuild)
	s.cache.UpdatePost(post.PostID, post.Content)

	return result, nil
}

func (s *PostService) DeletePost(ctx context.Context, authorID uuid.UUID, postID string) error {
	id, err := uuid.Parse(postID)
	if err != nil {
		return store.ErrPostNotFound
	}

	if err := s.store.Delete(ctx, id, authorID); err != nil {
		return err
	}

	// Immediately delete post from cache (no need for async worker)
	s.cache.DeletePost(id)

	return nil
}

func (s *PostService) Feed(ctx context.Context, userID uuid.UUID, query models.FeedQueryDTO) (models.FeedResponseDTO, error) {
	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	if posts := s.cache.Get(userID, limit, offset); posts != nil {
		slog.Debug("GETTING FEED FROM CACHE", "USERID", userID)
		return models.FeedResponseDTO{
			Posts:  posts,
			Source: "cache",
		}, nil
	}

	// Cache miss, fetch full feed and set cache
	fullPosts, err := s.store.Feed(ctx, userID, 1000, 0)
	if err != nil {
		return models.FeedResponseDTO{}, err
	}

	cachePosts := make([]models.GetPostDTO, 0, len(fullPosts))
	for _, p := range fullPosts {
		cachePosts = append(cachePosts, models.GetPostDTO{
			PostID:    p.PostID,
			AuthorID:  p.AuthorID,
			Content:   p.Content,
			CreatedAt: p.CreatedAt,
		})
	}
	s.cache.Set(userID, cachePosts)

	// Now return from cache
	return models.FeedResponseDTO{
		Posts:  s.cache.Get(userID, limit, offset),
		Source: "db",
	}, nil
}

func (s *PostService) RebuildFeed(ctx context.Context, userID uuid.UUID) error {
	return s.worker.RebuildUser(ctx, userID)
}

func (s *PostService) RebuildAllFeeds(ctx context.Context) error {
	s.worker.RebuildAll(ctx)
	return nil
}
