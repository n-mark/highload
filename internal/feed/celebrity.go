package feed

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"example.com/highload/myproject/internal/store"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// CelebrityResolver caches is_celebrity lookups with in-memory L1 and Redis L2.
type CelebrityResolver struct {
	friendStore  *store.FriendStore
	redis        *redis.Client
	threshold    int
	inMemTTL     time.Duration
	redisTTL     time.Duration

	mu     sync.RWMutex
	cache  map[string]celebCacheEntry
}

type celebCacheEntry struct {
	isCeleb bool
	expires time.Time
}

func NewCelebrityResolver(friendStore *store.FriendStore, redisClient *redis.Client, threshold int) *CelebrityResolver {
	return &CelebrityResolver{
		friendStore: friendStore,
		redis:       redisClient,
		threshold:   threshold,
		inMemTTL:    30 * time.Second,
		redisTTL:    5 * time.Minute,
		cache:       make(map[string]celebCacheEntry),
	}
}

func (r *CelebrityResolver) key(userID uuid.UUID) string {
	return "celeb:" + userID.String()
}

func (r *CelebrityResolver) IsCelebrity(ctx context.Context, userID uuid.UUID) (bool, error) {
	// L1 in-memory cache
	r.mu.RLock()
	if entry, ok := r.cache[userID.String()]; ok && entry.expires.After(time.Now()) {
		r.mu.RUnlock()
		return entry.isCeleb, nil
	}
	r.mu.RUnlock()

	// L2 Redis cache
	if r.redis != nil {
		val, err := r.redis.Get(ctx, r.key(userID)).Result()
		if err == nil {
			isCeleb := val == "1"
			r.mu.Lock()
			r.cache[userID.String()] = celebCacheEntry{isCeleb: isCeleb, expires: time.Now().Add(r.inMemTTL)}
			r.mu.Unlock()
			return isCeleb, nil
		}
		if err != redis.Nil {
			log.Printf("celebrity resolver: redis get error for %s: %v", userID, err)
		}
	}

	// L3 database (source of truth)
	isCeleb, err := r.friendStore.IsCelebrity(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("celebrity resolver: db lookup failed for %s: %w", userID, err)
	}

	// Write-back to caches
	r.mu.Lock()
	r.cache[userID.String()] = celebCacheEntry{isCeleb: isCeleb, expires: time.Now().Add(r.inMemTTL)}
	r.mu.Unlock()

	if r.redis != nil {
		val := "0"
		if isCeleb {
			val = "1"
		}
		if err := r.redis.Set(ctx, r.key(userID), val, r.redisTTL).Err(); err != nil {
			log.Printf("celebrity resolver: redis set error for %s: %v", userID, err)
		}
	}

	return isCeleb, nil
}

// Invalidate removes the user from all caches.
func (r *CelebrityResolver) Invalidate(userID uuid.UUID) {
	r.mu.Lock()
	delete(r.cache, userID.String())
	r.mu.Unlock()

	if r.redis != nil {
		ctx := context.Background()
		if err := r.redis.Del(ctx, r.key(userID)).Err(); err != nil {
			log.Printf("celebrity resolver: redis del error for %s: %v", userID, err)
		}
	}
}

// CelebrityFriendsOf returns celebrity friends of the given user.
func (r *CelebrityResolver) CelebrityFriendsOf(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return r.friendStore.GetCelebrityFriendIDs(ctx, userID)
}

// NonCelebrityFriendsOf returns non-celebrity friends of the given user.
func (r *CelebrityResolver) NonCelebrityFriendsOf(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return r.friendStore.GetNonCelebrityFriendIDs(ctx, userID)
}

// Cleanup runs periodic eviction of expired in-memory entries.
func (r *CelebrityResolver) Cleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for k, v := range r.cache {
				if v.expires.Before(now) {
					delete(r.cache, k)
				}
			}
			r.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}
