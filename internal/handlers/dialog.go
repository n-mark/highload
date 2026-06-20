package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"example.com/highload/myproject/internal/auth"
	"example.com/highload/myproject/internal/models"
	"github.com/google/uuid"
)

const (
    reqHeaderName = "X-Request-ID"
    userId        = "X-User-ID"
)

type DialogHandler struct {
    middleware         AuthMiddleware
    dialogServiceProps DialogSvcProps
    httpClient         *http.Client // Сделал указателем
}

func NewDialogHandler(middleware AuthMiddleware, dialogServiceProps DialogSvcProps) *DialogHandler {
    return &DialogHandler{
        middleware:         middleware,
        dialogServiceProps: dialogServiceProps,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
            },
        },
    }
}

func (h *DialogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

    h.extSvcCallSend(w, r, fromUserID, toUserID, dto)
}

func (h *DialogHandler) extSvcCallSend(w http.ResponseWriter, r *http.Request, fromUserID, toUserID uuid.UUID, dto models.SendMessageDTO) {
    // 1. Маршалим тело
    body, err := json.Marshal(dto)
    if err != nil {
        http.Error(w, fmt.Sprintf("failed to marshal body: %v", err), http.StatusInternalServerError)
        return
    }

    // 2. Получаем или генерируем Request ID
    reqId := r.Header.Get(reqHeaderName)
    if reqId == "" {
        reqId = uuid.New().String()
    }

	slog.Info("DIALOG SEND MSG ENDPOINT PROXY INVOKED.", "REQUESTID", reqId)

    // 3. Формируем URL
    endpoint := fmt.Sprintf(h.dialogServiceProps.DialogSvcSend, toUserID)
    reqAddr := fmt.Sprintf("%s://%s/%s", h.dialogServiceProps.DialogSvcProtocol, h.dialogServiceProps.DialogSvcAddr, endpoint)

    // 4. СОЗДАЕМ ЗАПРОС (с проверкой ошибок!)
    req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, reqAddr, bytes.NewReader(body))
    if err != nil {
        http.Error(w, fmt.Sprintf("failed to create request: %v", err), http.StatusInternalServerError)
        return
    }

    // 5. Ставим хедеры (теперь безопасно)
    req.Header.Set(reqHeaderName, reqId)
    req.Header.Set(userId, fromUserID.String())
    req.Header.Set("Content-Type", "application/json")

    // 6. ВЫПОЛНЯЕМ ЗАПРОС (с проверкой ошибок!)
    resp, err := h.httpClient.Do(req)
    if err != nil {
        http.Error(w, fmt.Sprintf("upstream request failed: %v", err), http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    // 7. Проверяем статус ответа!
    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
        bodyBytes, readErr := io.ReadAll(resp.Body)
        if readErr != nil {
            http.Error(w, fmt.Sprintf("upstream error: status %d", resp.StatusCode), resp.StatusCode)
            return
        }
        http.Error(w, fmt.Sprintf("upstream error: %s", string(bodyBytes)), resp.StatusCode)
        return
    }

    // 8. Отвечаем клиенту
    writeJSON(w, http.StatusCreated, map[string]string{"status": "CREATED"})
}

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

    msgs, err := h.extSvcCallList(r, requesterID, otherUserID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    writeJSON(w, http.StatusOK, msgs)
}

func (h *DialogHandler) extSvcCallList(r *http.Request, requesterID, otherUserID uuid.UUID) ([]models.GetMessageDTO, error) {
	host := fmt.Sprintf("%s://%s", h.dialogServiceProps.DialogSvcProtocol, h.dialogServiceProps.DialogSvcAddr)
    // 1. Формируем базовый URL
    baseURL, err := url.Parse(host)
    if err != nil {
        return nil, fmt.Errorf("invalid service address: %w", err)
    }

    endpoint := fmt.Sprintf(h.dialogServiceProps.DialogSvcList, otherUserID)
    baseURL.Path = endpoint

    // 2. Добавляем query-параметры из входящего запроса (если есть)
    query := baseURL.Query()

    // Пробрасываем параметры пагинации
    if limit := r.URL.Query().Get("limit"); limit != "" {
        query.Set("limit", limit)
    }
    if offset := r.URL.Query().Get("offset"); offset != "" {
        query.Set("offset", offset)
    }

    baseURL.RawQuery = query.Encode()
    reqAddr := baseURL.String()

    // 3. Создаем запрос
    req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, reqAddr, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    // 4. Хедеры
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set(userId, requesterID.String())

    if reqId := r.Header.Get(reqHeaderName); reqId != "" {
        req.Header.Set(reqHeaderName, reqId)
    }

    // 5. Выполняем запрос
    resp, err := h.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("upstream request failed: %w", err)
    }
    defer resp.Body.Close()

    // 6. Читаем и парсим ответ
    bodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("upstream error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
    }

    var messages []models.GetMessageDTO
    if err := json.Unmarshal(bodyBytes, &messages); err != nil {
        return nil, fmt.Errorf("failed to parse response JSON: %w", err)
    }

    return messages, nil
}