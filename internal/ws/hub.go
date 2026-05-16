package ws

import (
	"github.com/google/uuid"
)

const sendBufferSize = 256

// Hub orchestrates active WebSocket clients and delivers events to specific users.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMessage

	clients map[uuid.UUID]map[*Client]struct{}

	bindHandler   func(uuid.UUID)
	unbindHandler func(uuid.UUID)
}

// broadcastMessage holds payload data for a single user.
type broadcastMessage struct {
	userID  uuid.UUID
	payload []byte
}

// NewHub constructs a Hub ready for running.
func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcastMessage, 1024),
		clients:    make(map[uuid.UUID]map[*Client]struct{}),
	}
}

// Run starts the hub loop. It should be called once.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.addClient(client)
		case client := <-h.unregister:
			h.removeClient(client)
		case msg := <-h.broadcast:
			h.broadcastToUser(msg)
		}
	}
}

func (h *Hub) addClient(client *Client) {
	userClients, ok := h.clients[client.userID]
	if !ok {
		userClients = make(map[*Client]struct{})
		h.clients[client.userID] = userClients
	}
	userClients[client] = struct{}{}

	if len(userClients) == 1 && h.bindHandler != nil {
		h.bindHandler(client.userID)
	}
}

func (h *Hub) removeClient(client *Client) {
	clients, ok := h.clients[client.userID]
	if !ok {
		return
	}
	if _, exists := clients[client]; !exists {
		return
	}
	delete(clients, client)
	close(client.send)

	if len(clients) == 0 {
		delete(h.clients, client.userID)
		if h.unbindHandler != nil {
			h.unbindHandler(client.userID)
		}
	}
}

func (h *Hub) broadcastToUser(msg broadcastMessage) {
	clients, ok := h.clients[msg.userID]
	if !ok {
		return
	}
	for client := range clients {
		select {
		case client.send <- msg.payload:
		default:
			close(client.send)
			delete(clients, client)
		}
	}
}

// Register registers a client with the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes client from hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// BroadcastToUser sends payload to all open connections of a user.
func (h *Hub) BroadcastToUser(userID uuid.UUID, payload []byte) {
	h.broadcast <- broadcastMessage{userID: userID, payload: payload}
}

// SetBindHandler sets a callback invoked when the first connection of a user registers.
func (h *Hub) SetBindHandler(handler func(uuid.UUID)) {
	h.bindHandler = handler
}

// SetUnbindHandler sets a callback invoked when the last connection of a user unregisters.
func (h *Hub) SetUnbindHandler(handler func(uuid.UUID)) {
	h.unbindHandler = handler
}
