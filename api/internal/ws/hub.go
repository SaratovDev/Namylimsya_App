package ws

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Hub управляет всеми комнатами (event_id → clients).
type Hub struct {
	mu    sync.RWMutex
	rooms map[uuid.UUID]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[uuid.UUID]map[*Client]struct{})}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[c.EventID] == nil {
		h.rooms[c.EventID] = make(map[*Client]struct{})
	}
	h.rooms[c.EventID][c] = struct{}{}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room, ok := h.rooms[c.EventID]; ok {
		delete(room, c)
		if len(room) == 0 {
			delete(h.rooms, c.EventID)
		}
	}
}

func (h *Hub) Broadcast(eventID uuid.UUID, msg any) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	room := h.rooms[eventID]
	if len(room) == 0 {
		return
	}

	// Если пришли уже сериализованные байты — не маршалим повторно
	var data []byte
	switch v := msg.(type) {
	case []byte:
		data = v
	case json.RawMessage:
		data = v
	default:
		var err error
		data, err = json.Marshal(v)
		if err != nil {
			return
		}
	}

	for c := range room {
		c.Enqueue(data)
	}
}

// Client — одно WS-соединение.
type Client struct {
	Hub     *Hub
	Conn    *websocket.Conn
	EventID uuid.UUID
	UserID  uuid.UUID
	send    chan []byte
}

func NewClient(hub *Hub, conn *websocket.Conn, eventID, userID uuid.UUID) *Client {
	return &Client{
		Hub: hub, Conn: conn, EventID: eventID, UserID: userID,
		send: make(chan []byte, 32),
	}
}

func (c *Client) Enqueue(data []byte) {
	select {
	case c.send <- data:
	default:
		// канал переполнен — клиент не успевает читать, отключаем
		close(c.send)
	}
}

func (c *Client) Send() <-chan []byte { return c.send }
