package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"example.com/highload/myproject/internal/auth"
	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
)

// DialogService is the interface the handler depends on.
type DialogService interface {
	Send(ctx context.Context, fromUserID, toUserID uuid.UUID, dto models.SendMessageDTO) (models.GetMessageDTO, error)
	List(ctx context.Context, userA, userB uuid.UUID) ([]models.GetMessageDTO, error)
}

// DialogHandler handles /dialog/{user_id}/send and /dialog/{user_id}/list.
type DialogHandler struct {
	service    DialogService
	middleware AuthMiddleware
}

func NewDialogHandler(service DialogService, middleware AuthMiddleware) *DialogHandler {
	return &DialogHandler{service: service, middleware: middleware}
}

// ServeHTTP dispatches to Send or List based on the URL suffix.
// Routes registered in server.go:
//
//	POST /dialog/{user_id}/send
//	GET  /dialog/{user_id}/list
func (h *DialogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract the target user_id and action from the path.
	// Path format: /dialog/<user_id>/<action>
	path := strings.TrimPrefix(r.URL.Path, "/dialog/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid path", http.StatusNotFound)
		return
	}

	targetUserIDStr := parts[0]
	action := parts[1]

	targetUserID, err := uuid.Parse(targetUserIDStr)
	if err != nil {
		http.Error(w, "invalid user_id in path", http.StatusBadRequest)
		return
	}

	switch action {
	case "send":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.send(w, r, targetUserID)
	case "list":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.list(w, r, targetUserID)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// send handles POST /dialog/{user_id}/send
func (h *DialogHandler) send(w http.ResponseWriter, r *http.Request, toUserID uuid.UUID) {
	senderStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	fromUserID, err := uuid.Parse(senderStr)
	if err != nil {
		http.Error(w, "invalid authenticated user id", http.StatusUnauthorized)
		return
	}

	var dto models.SendMessageDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	msg, err := h.service.Send(r.Context(), fromUserID, toUserID, dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}

// list handles GET /dialog/{user_id}/list
func (h *DialogHandler) list(w http.ResponseWriter, r *http.Request, otherUserID uuid.UUID) {
	requesterStr, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	requesterID, err := uuid.Parse(requesterStr)
	if err != nil {
		http.Error(w, "invalid authenticated user id", http.StatusUnauthorized)
		return
	}

	msgs, err := h.service.List(r.Context(), requesterID, otherUserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, msgs)
}
