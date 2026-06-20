package feed

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"sort"
	"sync"

	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

type Worker struct {
	cache       *Cache
	friendStore *store.FriendStore
	postStore   *store.PostStore
	celebrity   *CelebrityResolver
	publisher   *RabbitPublisher
	writer      *kafka.Writer
	wg          sync.WaitGroup
	stop        chan struct{}
}

func NewWorker(cache *Cache, friendStore *store.FriendStore, postStore *store.PostStore, brokers []string, topic string, publisher *RabbitPublisher, celebrity *CelebrityResolver) *Worker {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	return &Worker{
		cache:       cache,
		friendStore: friendStore,
		postStore:   postStore,
		celebrity:   celebrity,
		publisher:   publisher,
		writer:      writer,
		stop:        make(chan struct{}),
	}
}

func (w *Worker) Start(workers int, brokers []string, topic string) {
	w.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go w.consumerLoop(brokers, topic, i)
	}
	// Start cleanup goroutine for in-memory celebrity cache
	go w.celebrity.Cleanup(context.Background())
	log.Printf("feed worker: started %d consumers", workers)
}

func (w *Worker) Stop() {
	close(w.stop)
	if err := w.writer.Close(); err != nil {
		log.Printf("feed worker: failed to close writer: %v", err)
	}
	w.wg.Wait()
	log.Println("feed worker: stopped")
}

func (w *Worker) Enqueue(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("feed worker: failed to marshal event: %v", err)
		return
	}
	slog.Debug("START KAFKA MESSAGING", "MESSAGE", data)
	msg := kafka.Message{
		Value: data,
	}
	slog.Debug("KAFKA MESSAGE SENT")

	if err := w.writer.WriteMessages(context.Background(), msg); err != nil {
		log.Printf("feed worker: failed to write message: %v", err)
	}
}

func (w *Worker) consumerLoop(brokers []string, topic string, id int) {
	defer w.wg.Done()
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  "feed-worker-group",
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})
	defer func() {
		if err := r.Close(); err != nil {
			log.Printf("consumer %d: failed to close reader: %v", id, err)
		}
	}()
	ctx := context.Background()
	for {
		select {
		case <-w.stop:
			return
		default:
			m, err := r.ReadMessage(ctx)
			if err != nil {
				log.Printf("consumer %d: failed to read message: %v", id, err)
				continue
			}
			var event Event
			if err := json.Unmarshal(m.Value, &event); err != nil {
				log.Printf("consumer %d: failed to unmarshal event: %v", id, err)
				continue
			}
			w.process(event)
		}
	}
}

func (w *Worker) process(event Event) {
	ctx := context.Background()
	switch event.Type {
	case EventPostCreated:
		w.handlePostCreated(ctx, event)
	case EventPostUpdated:
		w.handlePostUpdated(ctx, event)
	case EventPostDeleted:
		w.handlePostDeleted(ctx, event)
	case EventFriendAdded:
		w.handleFriendAdded(ctx, event)
	case EventFriendRemoved:
		w.handleFriendRemoved(ctx, event)
	case EventRebuildFeed:
		w.handleRebuildFeed(ctx, event)
	}
}

func (w *Worker) handlePostCreated(ctx context.Context, event Event) {
	post := models.GetPostDTO{
		PostID:    event.PostID,
		AuthorID:  event.AuthorID,
		Content:   event.Content,
		CreatedAt: event.CreatedAt,
	}

	isCeleb, err := w.celebrity.IsCelebrity(ctx, event.AuthorID)
	if err != nil {
		log.Printf("feed worker: celebrity check failed for %s: %v", event.AuthorID, err)
		isCeleb = false
	}

	if isCeleb {
		w.cache.InsertPost([]uuid.UUID{event.AuthorID}, post)
		w.cache.InsertCelebPost(event.AuthorID, post)

		if w.publisher != nil {
			if err := w.publisher.PublishPostFromCelebrity(event.AuthorID, post); err != nil {
				log.Printf("feed worker: failed to publish celeb post for %s: %v", event.AuthorID, err)
			}
		}
		return
	}

	followerIDs, err := w.friendStore.GetFollowers(ctx, event.AuthorID)
	if err != nil {
		log.Printf("feed worker: failed to get followers for %s: %v", event.AuthorID, err)
		return
	}

	allIDs := make([]uuid.UUID, 0, len(followerIDs)+1)
	allIDs = append(allIDs, event.AuthorID)
	allIDs = append(allIDs, followerIDs...)

	w.cache.InsertPost(allIDs, post)

	if w.publisher != nil {
		for _, uid := range allIDs {
			log.Printf("feed worker: publishing rabbit event to user %s", uid)
			if err := w.publisher.PublishPostForUser(uid, post); err != nil {
				log.Printf("feed worker: failed to publish rabbit event for %s: %v", uid, err)
			}
		}
	}
}

func (w *Worker) handlePostUpdated(ctx context.Context, event Event) {
	w.cache.UpdatePost(event.PostID, event.Content)
}

func (w *Worker) handlePostDeleted(ctx context.Context, event Event) {
	w.cache.DeletePost(event.PostID)

	isCeleb, err := w.celebrity.IsCelebrity(ctx, event.AuthorID)
	if err != nil {
		log.Printf("feed worker: celebrity check on delete for %s: %v", event.AuthorID, err)
		return
	}
	if isCeleb {
		w.cache.DeleteCelebPost(event.AuthorID, event.PostID)
	}
}

func (w *Worker) handleFriendAdded(ctx context.Context, event Event) {
	slog.Debug("ENTER HANDLE FRIEND ADD EVENT PROCESSING")
	w.rebuildUserFeed(ctx, event.FriendID)
}

func (w *Worker) handleFriendRemoved(ctx context.Context, event Event) {
	w.cache.Invalidate(event.FriendID)
}

func (w *Worker) handleRebuildFeed(ctx context.Context, event Event) {
	w.rebuildUserFeed(ctx, event.AuthorID)
}

func (w *Worker) rebuildUserFeed(ctx context.Context, userID uuid.UUID) {
	friendIDs, err := w.celebrity.NonCelebrityFriendsOf(ctx, userID)
	if err != nil {
		log.Printf("feed worker: failed to get non-celebrity friends for %s: %v", userID, err)
		return
	}

	allAuthorIDs := append([]uuid.UUID{userID}, friendIDs...)

	posts, err := w.postStore.FeedFromAuthors(ctx, allAuthorIDs, maxFeedSize, 0)
	if err != nil {
		log.Printf("feed worker: failed to rebuild feed for %s: %v", userID, err)
		return
	}
	dtos := make([]models.GetPostDTO, 0, len(posts))
	for _, p := range posts {
		dtos = append(dtos, models.GetPostDTO{
			PostID:    p.PostID,
			AuthorID:  p.AuthorID,
			Content:   p.Content,
			CreatedAt: p.CreatedAt,
		})
	}
	w.cache.Set(userID, dtos)
}

func (w *Worker) RebuildAll(ctx context.Context) {
	userIDs := w.cache.AllUserIDs()
	for _, uid := range userIDs {
		w.rebuildUserFeed(ctx, uid)
	}
	log.Printf("feed worker: rebuilt feeds for %d users", len(userIDs))
}

func (w *Worker) RebuildUser(ctx context.Context, userID uuid.UUID) error {
	w.rebuildUserFeed(ctx, userID)
	return nil
}

// mergeSortedByTime merges two sorted-by-time-desc slices of posts.
func mergeSortedByTime(a, b []models.GetPostDTO, limit int) []models.GetPostDTO {
	merged := make([]models.GetPostDTO, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) && len(merged) < limit {
		if a[i].CreatedAt.After(b[j].CreatedAt) || (a[i].CreatedAt.Equal(b[j].CreatedAt) && a[i].PostID.String() > b[j].PostID.String()) {
			merged = append(merged, a[i])
			i++
		} else {
			merged = append(merged, b[j])
			j++
		}
	}
	for i < len(a) && len(merged) < limit {
		merged = append(merged, a[i])
		i++
	}
	for j < len(b) && len(merged) < limit {
		merged = append(merged, b[j])
		j++
	}
	return merged
}

// FetchAndMergeFeed returns the merged feed for a user: push cache + celebrity posts.
func (w *Worker) FetchAndMergeFeed(ctx context.Context, userID uuid.UUID, limit, offset int) []models.GetPostDTO {
	base := w.cache.Get(userID, limit+offset, 0)

	celebs, err := w.celebrity.CelebrityFriendsOf(ctx, userID)
	if err != nil || len(celebs) == 0 {
		if offset < len(base) {
			end := offset + limit
			if end > len(base) {
				end = len(base)
			}
			return base[offset:end]
		}
		return nil
	}

	celebPosts := make([]models.GetPostDTO, 0, len(celebs)*20)
	for _, celebID := range celebs {
		posts := w.cache.GetCelebPosts(celebID, limit+offset)
		celebPosts = append(celebPosts, posts...)
	}

	sort.Slice(celebPosts, func(i, j int) bool {
		return celebPosts[i].CreatedAt.After(celebPosts[j].CreatedAt)
	})

	merged := mergeSortedByTime(base, celebPosts, limit+offset)
	if offset >= len(merged) {
		return nil
	}
	end := offset + limit
	if end > len(merged) {
		end = len(merged)
	}
	return merged[offset:end]
}
