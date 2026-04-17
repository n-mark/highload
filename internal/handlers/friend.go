package handlers

import (
	"encoding/json"
	"net/http"

	"example.com/highload/myproject/internal/auth"
	"example.com/highload/myproject/internal/models"
	"example.com/highload/myproject/internal/store"
	"github.com/google/uuid"
)

type FriendHandler struct {
	service    FriendService
	middleware AuthMiddleware
}

func NewFriendHandler(service FriendService, middleware AuthMiddleware) *FriendHandler {
	return &FriendHandler{service: service, middleware: middleware}
}

func (h *FriendHandler) AddFriend(w http.ResponseWriter, r *http.Request) {
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

	var dto models.FriendActionDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := h.service.AddFriend(r.Context(), userID, dto); err != nil {
		status := http.StatusBadRequest
		switch err {
		case store.ErrAlreadyFriends, store.ErrFriendSelf:
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FriendHandler) DeleteFriend(w http.ResponseWriter, r *http.Request) {
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

	var dto models.FriendActionDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteFriend(r.Context(), userID, dto); err != nil {
		status := http.StatusBadRequest
		switch err {
		case store.ErrFriendNotFound:
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
