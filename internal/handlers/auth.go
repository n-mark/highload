package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"example.com/highload/myproject/internal/models"
)

type AuthHandler struct {
	service AuthService
}

func NewAuthHandler(service AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var dto models.LoginDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	token, err := h.service.Login(r.Context(), dto)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	writeJSON(w, http.StatusOK, token)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var dto models.CreateUserDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	user, err := h.service.Register(r.Context(), dto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("user %s*** created", user.Username[:1])
	writeJSON(w, http.StatusCreated, user)
}

// VerifyHandler проверяет JWT и возвращает X-User-ID
// Traefik будет вызывать этот эндпоинт перед проксированием в Service B
func (h *AuthHandler) VerifyHandler(w http.ResponseWriter, r *http.Request) {
    // 1. Получаем токен из заголовка Authorization или cookie
    token := h.extractToken(r)
    if token == "" {
        http.Error(w, "missing token", http.StatusUnauthorized)
        return
    }

    // 2. Валидируем токен (твоя логика)
    userID, err := h.service.Validate(token)
	// validateJWT(token)
    if err != nil {
        http.Error(w, "invalid token", http.StatusUnauthorized)
        return
    }

    // 3. Успех! Возвращаем хедер X-User-ID
    // Traefik подхватит этот хедер и передаст в Service B
    w.Header().Set("X-User-ID", userID)
    w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"ok"}`))
}

func (h *AuthHandler) extractToken(r *http.Request) string {
    // Из Authorization header
    auth := r.Header.Get("Authorization")
    if auth != "" && strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }

    // Из cookie
    cookie, err := r.Cookie("token")
    if err == nil {
        return cookie.Value
    }

    return ""
}