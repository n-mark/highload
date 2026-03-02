package handlers

import (
	"encoding/json"
	"net/http"

	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
)

type UserHandler struct {
	service    UserService
	middleware AuthMiddleware
}

func NewUserHandler(service UserService, middleware AuthMiddleware) *UserHandler {
	return &UserHandler{service: service, middleware: middleware}
}

func (h *UserHandler) HandleUser(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.CreateUser(w, r)
	case http.MethodPut:
		h.middleware.RequireAuth(http.HandlerFunc(h.UpdateUser)).ServeHTTP(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var dto models.CreateUserDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	user, err := h.service.CreateUser(dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	var dto models.UpdateUserDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if dto.UserID == uuid.Nil {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	user, err := h.service.UpdateUser(dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, user)
}
