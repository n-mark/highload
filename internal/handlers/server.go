package handlers

import (
	"net/http"
)

type Server struct {
	userHandler    *UserHandler
	profileHandler *ProfileHandler
	authHandler    *AuthHandler
}

func NewServer(userService UserService, profileService ProfileService, authService AuthService, middleware AuthMiddleware) *Server {
	return &Server{
		userHandler:    NewUserHandler(userService, middleware),
		profileHandler: NewProfileHandler(profileService, middleware),
		authHandler:    NewAuthHandler(authService),
	}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/auth/login", s.authHandler.Login)
	mux.HandleFunc("/auth/register", s.authHandler.Register)

	mux.HandleFunc("/user", s.userHandler.HandleUser)
	mux.HandleFunc("/profile", s.profileHandler.HandleProfile)
	mux.HandleFunc("/profile/list", s.profileHandler.List)

	return mux
}
