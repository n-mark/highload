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
	store     *store.PostStore
	friendStore *store.FriendStore
	cache       *feed.Cache
	worker      *feed.Worker
	celebrity   *feed.CelebrityResolver
}

func NewPostService(store *store.PostStore, friendStore *store.FriendStore, cache *feed.Cache, worker *feed.Worker, celebrity *feed.CelebrityResolver) *PostService {
	return &PostService{store: store, friendStore: friendStore, cache: cache, worker: worker, celebrity: celebrity}
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

	// Enqueue event so feed worker handles celebrity vs non-celebrity logic asynchronously
	go s.worker.Enqueue(feed.Event{
		Type:      feed.EventPostCreated,
		PostID:    post.PostID,
		AuthorID:  post.AuthorID,
		Content:   post.Content,
		CreatedAt: post.CreatedAt,
	})

	// Pre-warm cache: immediately handle based on celebrity status
	go s.prewarmCacheForFollowers(post.PostID, post.AuthorID, post.Content, post.CreatedAt)

	return result, nil
}

// prewarmCacheForFollowers synchronously inserts the post into the cache for followers
func (s *PostService) prewarmCacheForFollowers(postID, authorID uuid.UUID, content string, createdAt time.Time) {
	ctx := context.Background()

	isCeleb, err := s.celebrity.IsCelebrity(ctx, authorID)
	if err != nil {
		slog.Error("prewarm cache: celebrity check failed", "authorID", authorID, "error", err)
		// fail-safe: treat as non-celebrity
		isCeleb = false
	}

	postDTO := models.GetPostDTO{
		PostID:    postID,
		AuthorID:  authorID,
		Content:   content,
		CreatedAt: createdAt,
	}

	if isCeleb {
		// Celebrity: only own feed + global celeb ZSET
		s.cache.InsertPost([]uuid.UUID{authorID}, postDTO)
		s.cache.InsertCelebPost(authorID, postDTO)
		slog.Debug("prewarm cache: inserted celeb post for self only")
		return
	}

	// Non-celebrity: full fan-out
	followerIDs, err := s.friendStore.GetFollowers(ctx, authorID)
	if err != nil {
		slog.Error("prewarm cache: failed to get friends", "authorID", authorID, "error", err)
		return
	}

	allIDs := make([]uuid.UUID, 0, len(followerIDs)+1)
	allIDs = append(allIDs, authorID)
	allIDs = append(allIDs, followerIDs...)

	if len(allIDs) == 0 {
		return
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

	s.cache.DeletePost(id)

	isCeleb, _ := s.celebrity.IsCelebrity(ctx, authorID)
	if isCeleb {
		s.cache.DeleteCelebPost(authorID, id)
	}

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

	// Use celebrity-aware merged feed
	posts := s.worker.FetchAndMergeFeed(ctx, userID, limit, offset)
	if posts != nil {
		slog.Debug("GETTING FEED FROM MERGED CACHE", "USERID", userID)
		return models.FeedResponseDTO{
			Posts:  posts,
			Source: "cache",
		}, nil
	}

	// Full DB fallback (rebuild including celebrity posts)
	celebFriends, err := s.celebrity.CelebrityFriendsOf(ctx, userID)
	if err != nil {
		slog.Warn("feed: failed to get celeb friends", "err", err)
	}

	nonCelebFriends, err := s.celebrity.NonCelebrityFriendsOf(ctx, userID)
	if err != nil {
		slog.Warn("feed: failed to get non-celeb friends", "err", err)
	}

	allAuthorIDs := append([]uuid.UUID{userID}, nonCelebFriends...)
	allAuthorIDs = append(allAuthorIDs, celebFriends...)

	fullPosts, err := s.store.FeedFromAuthors(ctx, allAuthorIDs, 1000, 0)
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
	// Only cache non-celebrity posts to keep push cache clean
	nonCelebPosts := make([]models.GetPostDTO, 0)
	for _, p := range cachePosts {
		isCeleb, _ := s.celebrity.IsCelebrity(ctx, p.AuthorID)
		if !isCeleb || p.AuthorID == userID {
			nonCelebPosts = append(nonCelebPosts, p)
		}
	}
	s.cache.Set(userID, nonCelebPosts)

	// Return paginated from full result
	end := offset + limit
	if end > len(cachePosts) {
		end = len(cachePosts)
	}
	if offset >= len(cachePosts) {
		return models.FeedResponseDTO{Posts: nil, Source: "db"}, nil
	}

	return models.FeedResponseDTO{
		Posts:  cachePosts[offset:end],
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
