package handlers

import (
	"net/http"

	"example.com/highload/myproject/internal/metrics"
	"example.com/highload/myproject/internal/store"
	"example.com/highload/myproject/internal/ws"
)

type Server struct {
	userHandler    *UserHandler
	profileHandler *ProfileHandler
	authHandler    *AuthHandler
	statsHandler   *StatsHandler
	friendHandler  *FriendHandler
	postHandler    *PostHandler
	dialogHandler  *DialogHandler
	wsHandler      *ws.WSHandler
	middleware     AuthMiddleware
}

type DialogSvcProps struct {
	DialogSvcAddr     string
	DialogSvcSend     string
	DialogSvcList     string
	DialogSvcProtocol string
}

func NewServer(userService UserService, profileService ProfileService,
	authService AuthService, middleware AuthMiddleware, replica *store.ReplicaPool,
	friendService FriendService, postService PostService, wsHandler *ws.WSHandler,
	dialogServiceProps DialogSvcProps) *Server {
	return &Server{
		userHandler:    NewUserHandler(userService, middleware),
		profileHandler: NewProfileHandler(profileService, middleware),
		authHandler:    NewAuthHandler(authService),
		statsHandler:   NewStatsHandler(replica),
		friendHandler:  NewFriendHandler(friendService, middleware),
		postHandler:    NewPostHandler(postService, middleware),
		dialogHandler:  NewDialogHandler(middleware, dialogServiceProps),
		wsHandler:      wsHandler,
		middleware:     middleware,
	}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/auth/login", s.authHandler.Login)
	mux.HandleFunc("/auth/register", s.authHandler.Register)
	mux.HandleFunc("/auth/verify", s.authHandler.VerifyHandler)
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

	mux.Handle("/post/rebuild", s.middleware.RequireAuth(http.HandlerFunc(s.postHandler.RebuildFeed)))
	mux.Handle("/post/rebuild-all", s.middleware.RequireAuth(http.HandlerFunc(s.postHandler.RebuildAllFeeds)))

	// Dialog routes — sharded via Citus
	// POST /dialog/{user_id}/send
	// GET  /dialog/{user_id}/list
	// WebSocket endpoint for feed updates
	mux.Handle("/post/feed/posted", s.wsHandler)
	mux.Handle("/dialog/", s.middleware.RequireAuth(s.dialogHandler))

	return metrics.Middleware(mux)
}
