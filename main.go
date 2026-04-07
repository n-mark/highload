package main

import (
	"context"
	"log"
	"net/http"

	"example.com/highload/myproject/internal/auth"
	"example.com/highload/myproject/internal/config"
	"example.com/highload/myproject/internal/handlers"
	"example.com/highload/myproject/internal/services"
	"example.com/highload/myproject/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
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

	passwordHasher := auth.NewBcryptHasher()
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, "myproject", "myproject-api")

	userService := services.NewUserService(userStore, passwordHasher)
	profileService := services.NewProfileService(profileStore)
	authService := services.NewAuthService(userStore, passwordHasher, jwtManager)

	middleware := auth.NewMiddleware(jwtManager)

	server := handlers.NewServer(userService, profileService, authService, middleware)

	log.Printf("listening on %s", cfg.ServerAddr)
	if err := http.ListenAndServe(cfg.ServerAddr, server.Router()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
