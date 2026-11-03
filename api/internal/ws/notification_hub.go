package ws

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type NotificationHub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]map[*NotificationClient]struct{}
}

func NewNotificationHub() *NotificationHub {
	return &NotificationHub{clients: make(map[uuid.UUID]map[*NotificationClient]struct{})}
}

func (h *NotificationHub) Register(c *NotificationClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.UserID] == nil {
		h.clients[c.UserID] = make(map[*NotificationClient]struct{})
	}
	h.clients[c.UserID][c] = struct{}{}
}

func (h *NotificationHub) Unregister(c *NotificationClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room, ok := h.clients[c.UserID]; ok {
		delete(room, c)
		if len(room) == 0 {
			delete(h.clients, c.UserID)
		}
	}
}

func (h *NotificationHub) SendToUser(userID uuid.UUID, payload any) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	room := h.clients[userID]
	if len(room) == 0 {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	for c := range room {
		c.Enqueue(data)
	}
}

type NotificationClient struct {
	Hub    *NotificationHub
	Conn   *websocket.Conn
	UserID uuid.UUID
	send   chan []byte
}

func NewNotificationClient(hub *NotificationHub, conn *websocket.Conn, userID uuid.UUID) *NotificationClient {
	return &NotificationClient{
		Hub: hub, Conn: conn, UserID: userID,
		send: make(chan []byte, 32),
	}
}

func (c *NotificationClient) Enqueue(data []byte) {
	select {
	case c.send <- data:
	default:
		close(c.send)
	}
}

func (c *NotificationClient) Send() <-chan []byte { return c.send }
