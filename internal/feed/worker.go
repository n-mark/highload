package feed

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
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
	writer      *kafka.Writer
	wg          sync.WaitGroup
	stop        chan struct{}
}

func NewWorker(cache *Cache, friendStore *store.FriendStore, postStore *store.PostStore, brokers []string, topic string) *Worker {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	return &Worker{
		cache:       cache,
		friendStore: friendStore,
		postStore:   postStore,
		writer:      writer,
		stop:        make(chan struct{}),
	}
}

func (w *Worker) Start(workers int, brokers []string, topic string) {
	w.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go w.consumerLoop(brokers, topic, i)
	}
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
	// Get all followers of the author (users who have authorID as friend)
	followerIDs, err := w.friendStore.GetFollowers(ctx, event.AuthorID)
	if err != nil {
		log.Printf("feed worker: failed to get followers for %s: %v", event.AuthorID, err)
		return
	}

	// Also include the author themselves
	allIDs := make([]uuid.UUID, 0, len(followerIDs)+1)
	allIDs = append(allIDs, followerIDs...)
	allIDs = append(allIDs, event.AuthorID)

	post := models.GetPostDTO{
		PostID:    event.PostID,
		AuthorID:  event.AuthorID,
		Content:   event.Content,
		CreatedAt: event.CreatedAt,
	}
	w.cache.InsertPost(allIDs, post)
}

func (w *Worker) handlePostUpdated(ctx context.Context, event Event) {
	w.cache.UpdatePost(event.PostID, event.Content)
}

func (w *Worker) handlePostDeleted(ctx context.Context, event Event) {
	w.cache.DeletePost(event.PostID)
}

func (w *Worker) handleFriendAdded(ctx context.Context, event Event) {
	slog.Debug("ENTER HANDLE FRIEND ADD EVENT PROCESSING")
	// user added friendID as friend -> rebuild user's feed to include friend's posts
	w.rebuildUserFeed(ctx, event.FriendID)
}

func (w *Worker) handleFriendRemoved(ctx context.Context, event Event) {
	// user removed friendID as friend -> invalidate cache, will rebuild on next access
	w.cache.Invalidate(event.FriendID)
}

func (w *Worker) handleRebuildFeed(ctx context.Context, event Event) {
	w.rebuildUserFeed(ctx, event.AuthorID)
}

func (w *Worker) rebuildUserFeed(ctx context.Context, userID uuid.UUID) {
	posts, err := w.postStore.Feed(ctx, userID, maxFeedSize, 0)
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

// RebuildAll rebuilds feeds for all users who have cached feeds.
func (w *Worker) RebuildAll(ctx context.Context) {
	// Get all user IDs from cache
	userIDs := w.cache.AllUserIDs()
	for _, uid := range userIDs {
		w.rebuildUserFeed(ctx, uid)
	}
	log.Printf("feed worker: rebuilt feeds for %d users", len(userIDs))
}

// RebuildUser rebuilds feed for a specific user (public method for admin endpoint).
func (w *Worker) RebuildUser(ctx context.Context, userID uuid.UUID) error {
	w.rebuildUserFeed(ctx, userID)
	return nil
}
