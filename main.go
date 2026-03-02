package main

import (
	"log"
	"net/http"

	"example.com/highload/myproject/internal/auth"
	"example.com/highload/myproject/internal/handlers"
	"example.com/highload/myproject/internal/services"
	"example.com/highload/myproject/internal/store"
)

func main() {
	userStore := store.NewUserStore()
	profileStore := store.NewProfileStore()

	passwordHasher := auth.NewBcryptHasher()
	jwtManager := auth.NewJWTManager("change-me-secret", "myproject", "myproject-api")

	userService := services.NewUserService(userStore, passwordHasher)
	profileService := services.NewProfileService(profileStore)
	authService := services.NewAuthService(userStore, passwordHasher, jwtManager)

	middleware := auth.NewMiddleware(jwtManager)

	server := handlers.NewServer(userService, profileService, authService, middleware)

	log.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", server.Router()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
