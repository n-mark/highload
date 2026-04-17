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
    friendHandler  *FriendHandler
    postHandler    *PostHandler
    middleware      AuthMiddleware
}

func NewServer(userService UserService, profileService ProfileService, authService AuthService, middleware AuthMiddleware, replica *store.ReplicaPool, friendService FriendService, postService PostService) *Server {
    return &Server{
        userHandler:    NewUserHandler(userService, middleware),
        profileHandler: NewProfileHandler(profileService, middleware),
        authHandler:    NewAuthHandler(authService),
        statsHandler:   NewStatsHandler(replica),
        friendHandler:  NewFriendHandler(friendService, middleware),
        postHandler:    NewPostHandler(postService, middleware),
        middleware:      middleware,
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

    mux.Handle("/friend/add", s.middleware.RequireAuth(http.HandlerFunc(s.friendHandler.AddFriend)))
    mux.Handle("/friend/delete", s.middleware.RequireAuth(http.HandlerFunc(s.friendHandler.DeleteFriend)))

    mux.Handle("/post/create", s.middleware.RequireAuth(http.HandlerFunc(s.postHandler.CreatePost)))
    mux.Handle("/post/update", s.middleware.RequireAuth(http.HandlerFunc(s.postHandler.UpdatePost)))
    mux.Handle("/post/delete", s.middleware.RequireAuth(http.HandlerFunc(s.postHandler.DeletePost)))
    mux.HandleFunc("/post/get", s.postHandler.GetPost)
    mux.Handle("/post/feed", s.middleware.RequireAuth(http.HandlerFunc(s.postHandler.Feed)))

    return metrics.Middleware(mux)
}
