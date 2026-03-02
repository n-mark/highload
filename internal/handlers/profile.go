package handlers

import (
	"encoding/json"
	"net/http"

	"example.com/highload/myproject/internal/auth"
	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
)

type ProfileHandler struct {
	service    ProfileService
	middleware AuthMiddleware
}

func NewProfileHandler(service ProfileService, middleware AuthMiddleware) *ProfileHandler {
	return &ProfileHandler{service: service, middleware: middleware}
}

func (h *ProfileHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.GetProfile(w, r)
	case http.MethodPost:
		h.middleware.RequireAuth(http.HandlerFunc(h.CreateProfile)).ServeHTTP(w, r)
	case http.MethodPut:
		h.middleware.RequireAuth(http.HandlerFunc(h.UpdateProfile)).ServeHTTP(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	profileIDStr := r.URL.Query().Get("profile_id")
	if profileIDStr == "" {
		http.Error(w, "profile_id is required", http.StatusBadRequest)
		return
	}

	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		http.Error(w, "invalid profile_id", http.StatusBadRequest)
		return
	}

	profile, err := h.service.GetOne(profileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (h *ProfileHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var query models.QueryDTO
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	profiles := h.service.List(query)
	writeJSON(w, http.StatusOK, profiles)
}

func (h *ProfileHandler) CreateProfile(w http.ResponseWriter, r *http.Request) {
	var dto models.ProfileDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ownerID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusUnauthorized)
		return
	}

	profile, err := h.service.Create(ownerID, dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, profile)
}

func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	profileIDStr := r.URL.Query().Get("profile_id")
	if profileIDStr == "" {
		http.Error(w, "profile_id is required", http.StatusBadRequest)
		return
	}

	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		http.Error(w, "invalid profile_id", http.StatusBadRequest)
		return
	}

	var dto models.ProfileDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ownerID, err := uuid.Parse(userID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusUnauthorized)
		return
	}

	profile, err := h.service.Update(ownerID, profileID, dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, profile)
}
