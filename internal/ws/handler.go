package ws

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"example.com/highload/myproject/internal/auth"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// Client represents a single WebSocket connection.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID uuid.UUID
}

// WSHandler handles WebSocket upgrade and client registration.
type WSHandler struct {
	hub        *Hub
	jwtManager *auth.JWTManager
	upgrader   websocket.Upgrader
}

// NewWSHandler creates a new WSHandler with the given hub and JWT manager.
func NewWSHandler(hub *Hub, jwt *auth.JWTManager) *WSHandler {
	return &WSHandler{
		hub:        hub,
		jwtManager: jwt,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// ServeHTTP upgrades the HTTP connection to WebSocket and registers the client.
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Authenticate via token query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token query param required", http.StatusUnauthorized)
		return
	}
	userStr, err := h.jwtManager.VerifyToken(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	userID, err := uuid.Parse(userStr)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error user=%s: %v", userID, err)
		http.Error(w, fmt.Sprintf("upgrade error: %v", err), http.StatusInternalServerError)
		return
	}
	client := &Client{hub: h.hub, conn: conn, send: make(chan []byte, sendBufferSize), userID: userID}
	h.hub.Register(client)

	// Start pumps
	go client.writePump()
	client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
