package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"example.com/highload/myproject/internal/auth"
	"example.com/highload/myproject/internal/config"
	"example.com/highload/myproject/internal/feed"
	"example.com/highload/myproject/internal/handlers"
	"example.com/highload/myproject/internal/services"
	"example.com/highload/myproject/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func initLogger() {
	level := slog.LevelInfo
	if strings.ToLower(os.Getenv("LOG_LEVEL")) == "debug" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})))
}

func main() {
	initLogger()

	cfg := config.Load()

	db, err := pgxpool.New(context.Background(), cfg.DSN())
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		log.Fatalf("database is not reachable: %v", err)
	}
	log.Println("connected to database")

	replicaPools := make([]*pgxpool.Pool, 0, len(cfg.ReplicaDSNs()))
	for _, dsn := range cfg.ReplicaDSNs() {
		pool, err := pgxpool.New(context.Background(), dsn)
		if err != nil {
			log.Fatalf("failed to connect to replica %s: %v", dsn, err)
		}
		if err := pool.Ping(context.Background()); err != nil {
			log.Fatalf("replica %s is not reachable: %v", dsn, err)
		}
		replicaPools = append(replicaPools, pool)
		log.Printf("connected to replica %s", dsn)
	}
	replicaDB := store.NewReplicaPool(replicaPools)
	defer replicaDB.Close()

	userStore := store.NewUserStore(db, replicaDB)
	profileStore := store.NewProfileStore(db, replicaDB)
	friendStore := store.NewFriendStore(db, replicaDB)
	postStore := store.NewPostStore(db, replicaDB)

	passwordHasher := auth.NewBcryptHasher()
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, "myproject", "myproject-api")

	userService := services.NewUserService(userStore, passwordHasher)
	profileService := services.NewProfileService(profileStore)
	authService := services.NewAuthService(userStore, passwordHasher, jwtManager)

	// Feed cache and worker
	feedCache := feed.NewCache(cfg.RedisAddr)
	feedWorker := feed.NewWorker(feedCache, friendStore, postStore, cfg.KafkaBrokerList(), "feed-events")
	feedWorker.Start(4, cfg.KafkaBrokerList(), "feed-events")
	defer feedWorker.Stop()

	friendService := services.NewFriendService(friendStore, feedWorker)
	postService := services.NewPostService(postStore, feedCache, feedWorker)

	middleware := auth.NewMiddleware(jwtManager)

	server := handlers.NewServer(userService, profileService, authService, middleware, replicaDB, friendService, postService)

	log.Printf("listening on %s", cfg.ServerAddr)
	if err := http.ListenAndServe(cfg.ServerAddr, server.Router()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
