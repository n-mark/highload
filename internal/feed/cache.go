package feed

import (
	"context"
	"encoding/json"
	"log"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const maxFeedSize = 1000

type Cache struct {
	rdb *redis.Client
}

func NewCache(addr string) *Cache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	return &Cache{
		rdb: rdb,
	}
}

func (c *Cache) keyFeeder(userID uuid.UUID) string {
	return "feeder:" + userID.String()
}

func (c *Cache) keyPost(postID uuid.UUID) string {
	return "post:" + postID.String()
}

func (c *Cache) keyPostUsers(postID uuid.UUID) string {
	return "post_users:" + postID.String()
}

func (c *Cache) keyUsersWithFeeds() string {
	return "users_with_feeds"
}

func (c *Cache) Get(userID uuid.UUID, limit, offset int) []models.GetPostDTO {
	ctx := context.Background()
	key := c.keyFeeder(userID)
	zrangeRes := c.rdb.ZRevRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    "+inf",
		Offset: int64(offset),
		Count:  int64(limit),
	})
	if zrangeRes.Err() != nil {
		log.Printf("cache get error: %v", zrangeRes.Err())
		return nil
	}
	postIDsStr := zrangeRes.Val()
	if len(postIDsStr) == 0 {
		return nil
	}
	postIDs := make([]string, len(postIDsStr))
	for i, id := range postIDsStr {
		postIDs[i] = "post:" + id
	}
	mgetRes := c.rdb.MGet(ctx, postIDs...)
	if mgetRes.Err() != nil {
		log.Printf("cache mget error: %v", mgetRes.Err())
		return nil
	}
	posts := make([]models.GetPostDTO, 0, len(postIDsStr))
	for _, val := range mgetRes.Val() {
		if val != nil {
			var post models.GetPostDTO
			if err := json.Unmarshal([]byte(val.(string)), &post); err == nil {
				posts = append(posts, post)
			}
		}
	}
	return posts
}

func (c *Cache) Set(userID uuid.UUID, posts []models.GetPostDTO) {
	ctx := context.Background()
	key := c.keyFeeder(userID)
	pipe := c.rdb.TxPipeline()
	pipe.Del(ctx, key)
	zvals := make([]redis.Z, 0, len(posts))
	for _, p := range posts {
		zvals = append(zvals, redis.Z{Score: float64(p.CreatedAt.Unix()), Member: p.PostID.String()})
		jsonData, _ := json.Marshal(p)
		pipe.Set(ctx, c.keyPost(p.PostID), jsonData, 0)
	}
	if len(zvals) > 0 {
		pipe.ZAdd(ctx, key, zvals...)
	}
	pipe.ZRemRangeByRank(ctx, key, 0, int64(len(posts)-maxFeedSize-1))
	pipe.SAdd(ctx, c.keyUsersWithFeeds(), userID.String())
	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("cache set error: %v", err)
	}
}

func (c *Cache) InsertPost(userIDs []uuid.UUID, post models.GetPostDTO) {
	ctx := context.Background()
	postKey := c.keyPost(post.PostID)
	usersKey := c.keyPostUsers(post.PostID)
	jsonData, _ := json.Marshal(post)
	score := float64(post.CreatedAt.Unix())
	pipe := c.rdb.TxPipeline()
	pipe.Set(ctx, postKey, jsonData, 0)
	for _, uid := range userIDs {
		feederKey := c.keyFeeder(uid)
		pipe.ZAdd(ctx, feederKey, redis.Z{Score: score, Member: post.PostID.String()})
		pipe.ZRemRangeByRank(ctx, feederKey, 0, -maxFeedSize-1) // keep latest
		pipe.SAdd(ctx, c.keyUsersWithFeeds(), uid.String())
	}
	pipe.SAdd(ctx, usersKey, userIDsToInterfaces(userIDs)...)
	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("cache insert post error: %v", err)
	}
}

func (c *Cache) UpdatePost(postID uuid.UUID, content string) {
	ctx := context.Background()
	postKey := c.keyPost(postID)
	getRes := c.rdb.Get(ctx, postKey)
	if getRes.Err() == redis.Nil {
		return
	}
	if getRes.Err() != nil {
		log.Printf("cache update post get error: %v", getRes.Err())
		return
	}
	var post models.GetPostDTO
	if err := json.Unmarshal([]byte(getRes.Val()), &post); err != nil {
		log.Printf("cache update unmarshal error: %v", err)
		return
	}
	post.Content = content
	jsonData, _ := json.Marshal(post)
	setRes := c.rdb.Set(ctx, postKey, jsonData, 0)
	if setRes.Err() != nil {
		log.Printf("cache update post set error: %v", setRes.Err())
	}
}

func (c *Cache) DeletePost(postID uuid.UUID) {
	ctx := context.Background()
	usersKey := c.keyPostUsers(postID)
	smembersRes := c.rdb.SMembers(ctx, usersKey)
	if smembersRes.Err() != nil {
		log.Printf("cache delete post smembers error: %v", smembersRes.Err())
		return
	}
	uidStrs := smembersRes.Val()
	pipe := c.rdb.TxPipeline()
	for _, uidStr := range uidStrs {
		if uid, err := uuid.Parse(uidStr); err == nil {
			feederKey := c.keyFeeder(uid)
			pipe.ZRem(ctx, feederKey, postID.String())
		}
	}
	pipe.Del(ctx, c.keyPost(postID))
	pipe.Del(ctx, usersKey)
	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("cache delete post error: %v", err)
	}
}

func (c *Cache) Invalidate(userID uuid.UUID) {
	ctx := context.Background()
	feederKey := c.keyFeeder(userID)
	delRes := c.rdb.Del(ctx, feederKey)
	if delRes.Err() != nil {
		log.Printf("cache invalidate error: %v", delRes.Err())
	}
}

func (c *Cache) AllUserIDs() []uuid.UUID {
	ctx := context.Background()
	smembersRes := c.rdb.SMembers(ctx, c.keyUsersWithFeeds())
	if smembersRes.Err() != nil {
		log.Printf("cache all user ids error: %v", smembersRes.Err())
		return nil
	}
	strs := smembersRes.Val()
	ids := make([]uuid.UUID, 0, len(strs))
	for _, s := range strs {
		if uid, err := uuid.Parse(s); err == nil {
			ids = append(ids, uid)
		}
	}
	return ids
}

func (c *Cache) Has(userID uuid.UUID) bool {
	ctx := context.Background()
	existsRes := c.rdb.Exists(ctx, c.keyFeeder(userID))
	if existsRes.Err() != nil {
		return false
	}
	return existsRes.Val() > 0
}

func userIDsToStrings(userIDs []uuid.UUID) []string {
	strs := make([]string, len(userIDs))
	for i, uid := range userIDs {
		strs[i] = uid.String()
	}
	return strs
}

func userIDsToInterfaces(userIDs []uuid.UUID) []interface{} {
	ifs := make([]interface{}, len(userIDs))
	for i, uid := range userIDs {
		ifs[i] = uid.String()
	}
	return ifs
}
