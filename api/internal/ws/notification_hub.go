package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type NotificationHub struct {
	mu         sync.RWMutex
	clients    map[uuid.UUID]map[*NotificationClient]struct{}
	maxPerUser int
}

func NewNotificationHub(maxPerUser int) *NotificationHub {
	return &NotificationHub{
		clients:    make(map[uuid.UUID]map[*NotificationClient]struct{}),
		maxPerUser: maxPerUser,
	}
}

// Register — см. Hub.Register: тот же LRU-принцип.
func (h *NotificationHub) Register(c *NotificationClient) {
	h.mu.Lock()

	if h.clients[c.UserID] == nil {
		h.clients[c.UserID] = make(map[*NotificationClient]struct{})
	}
	h.clients[c.UserID][c] = struct{}{}

	var toEvict *NotificationClient
	if h.maxPerUser > 0 && len(h.clients[c.UserID]) > h.maxPerUser {
		oldest := time.Time{}
		for x := range h.clients[c.UserID] {
			if x == c {
				continue
			}
			if oldest.IsZero() || x.ConnectedAt.Before(oldest) {
				oldest = x.ConnectedAt
				toEvict = x
			}
		}
		if toEvict != nil {
			delete(h.clients[c.UserID], toEvict)
		}
	}

	h.mu.Unlock()

	if toEvict != nil {
		toEvict.CloseWithCode(CloseReplaced, "replaced by newer connection")
	}
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
	Hub         *NotificationHub
	Conn        *websocket.Conn
	UserID      uuid.UUID
	ConnectedAt time.Time

	send      chan []byte
	mu        sync.Mutex
	closed    bool
	closeCode int
}

func NewNotificationClient(hub *NotificationHub, conn *websocket.Conn, userID uuid.UUID) *NotificationClient {
	return &NotificationClient{
		Hub:         hub,
		Conn:        conn,
		UserID:      userID,
		ConnectedAt: time.Now(),
		send:        make(chan []byte, 32),
	}
}

func (c *NotificationClient) Enqueue(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.send <- data:
	default:
		c.closed = true
		c.closeCode = websocket.ClosePolicyViolation
		close(c.send)
	}
}

func (c *NotificationClient) CloseWithCode(code int, _ string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	c.closeCode = code
	close(c.send)
}

func (c *NotificationClient) Send() <-chan []byte { return c.send }

func (c *NotificationClient) CloseCode() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeCode
}
