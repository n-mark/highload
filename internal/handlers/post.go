package handlers

import (
	"encoding/json"
	"net/http"

	"example.com/highload/myproject/internal/auth"
	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
	"github.com/google/uuid"
)

type PostHandler struct {
	service    PostService
	middleware AuthMiddleware
}

func NewPostHandler(service PostService, middleware AuthMiddleware) *PostHandler {
	return &PostHandler{service: service, middleware: middleware}
}

func (h *PostHandler) HandlePost(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.CreatePost(w, r)
	case http.MethodPut:
		h.UpdatePost(w, r)
	case http.MethodDelete:
		h.DeletePost(w, r)
	case http.MethodGet:
		h.GetPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PostHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	userStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	authorID, err := uuid.Parse(userStr)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusUnauthorized)
		return
	}

	var dto models.CreatePostDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	post, err := h.service.CreatePost(r.Context(), authorID, dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, post)
}

func (h *PostHandler) GetPost(w http.ResponseWriter, r *http.Request) {
	postID := r.URL.Query().Get("post_id")
	if postID == "" {
		http.Error(w, "post_id is required", http.StatusBadRequest)
		return
	}

	post, err := h.service.GetPost(r.Context(), postID)
	if err != nil {
		status := http.StatusBadRequest
		if err == store.ErrPostNotFound {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (h *PostHandler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	userStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	authorID, err := uuid.Parse(userStr)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusUnauthorized)
		return
	}

	var dto models.UpdatePostDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	post, err := h.service.UpdatePost(r.Context(), authorID, dto)
	if err != nil {
		status := http.StatusBadRequest
		if err == store.ErrPostNotFound {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	writeJSON(w, http.StatusOK, post)
}

func (h *PostHandler) DeletePost(w http.ResponseWriter, r *http.Request) {
	userStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	authorID, err := uuid.Parse(userStr)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusUnauthorized)
		return
	}

	postID := r.URL.Query().Get("post_id")
	if postID == "" {
		http.Error(w, "post_id is required", http.StatusBadRequest)
		return
	}

	if err := h.service.DeletePost(r.Context(), authorID, postID); err != nil {
		status := http.StatusBadRequest
		if err == store.ErrPostNotFound {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *PostHandler) Feed(w http.ResponseWriter, r *http.Request) {
	userStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	userID, err := uuid.Parse(userStr)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusUnauthorized)
		return
	}

	var query models.FeedQueryDTO
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		if val, err := json.Number(v).Int64(); err == nil {
			query.Limit = int(val)
		}
	}
	if v := q.Get("offset"); v != "" {
		if val, err := json.Number(v).Int64(); err == nil {
			query.Offset = int(val)
		}
	}

	posts, err := h.service.Feed(r.Context(), userID, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, posts)
}