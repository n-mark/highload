package handlers

import (
	"net/http"

	"example.com/highload/myproject/internal/metrics"
	"example.com/highload/myproject/internal/store"
)

type Server struct {
	userHandler    *UserHandler
	profileHandler *ProfileHandler
	authHandler    *AuthHandler
	statsHandler   *StatsHandler
}

func NewServer(userService UserService, profileService ProfileService, authService AuthService, middleware AuthMiddleware, replica *store.ReplicaPool) *Server {
	return &Server{
		userHandler:    NewUserHandler(userService, middleware),
		profileHandler: NewProfileHandler(profileService, middleware),
		authHandler:    NewAuthHandler(authService),
		statsHandler:   NewStatsHandler(replica),
	}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/auth/login", s.authHandler.Login)
	mux.HandleFunc("/auth/register", s.authHandler.Register)
	mux.HandleFunc("/stats", s.statsHandler.Handle)

	mux.HandleFunc("/user", s.userHandler.HandleUser)
	mux.HandleFunc("/profile", s.profileHandler.HandleProfile)
	mux.HandleFunc("/profile/list", s.profileHandler.List)

	return metrics.Middleware(mux)
}
