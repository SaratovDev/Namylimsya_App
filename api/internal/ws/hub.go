package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// CloseReplaced — close-code для LRU-эвикции (юзер превысил maxPerUser).
// Совпадает с application-кодом, который видит клиент (use-chat.ts).
const CloseReplaced = 4408

// Hub управляет комнатами (event_id → clients) и per-user connection cap.
// LRU: при превышении maxPerUser закрываем СТАРЕЙШЕЕ соединение этого
// юзера с close-code 4408, новое принимаем.
type Hub struct {
	mu         sync.RWMutex
	rooms      map[uuid.UUID]map[*Client]struct{}
	byUser     map[uuid.UUID]map[*Client]struct{}
	maxPerUser int
}

func NewHub(maxPerUser int) *Hub {
	return &Hub{
		rooms:      make(map[uuid.UUID]map[*Client]struct{}),
		byUser:     make(map[uuid.UUID]map[*Client]struct{}),
		maxPerUser: maxPerUser,
	}
}

// Register добавляет клиента в комнату и в byUser. Если у юзера уже
// maxPerUser соединений — отцепляет старейшее из maps и асинхронно
// инициирует его закрытие (writer-loop в Handle отправит CloseMessage 4408).
func (h *Hub) Register(c *Client) {
	h.mu.Lock()

	if h.rooms[c.EventID] == nil {
		h.rooms[c.EventID] = make(map[*Client]struct{})
	}
	h.rooms[c.EventID][c] = struct{}{}

	if h.byUser[c.UserID] == nil {
		h.byUser[c.UserID] = make(map[*Client]struct{})
	}
	h.byUser[c.UserID][c] = struct{}{}

	var toEvict *Client
	if h.maxPerUser > 0 && len(h.byUser[c.UserID]) > h.maxPerUser {
		oldest := time.Time{}
		for x := range h.byUser[c.UserID] {
			if x == c {
				continue
			}
			if oldest.IsZero() || x.ConnectedAt.Before(oldest) {
				oldest = x.ConnectedAt
				toEvict = x
			}
		}
		if toEvict != nil {
			delete(h.byUser[c.UserID], toEvict)
			if room := h.rooms[toEvict.EventID]; room != nil {
				delete(room, toEvict)
				if len(room) == 0 {
					delete(h.rooms, toEvict.EventID)
				}
			}
		}
	}

	h.mu.Unlock()

	if toEvict != nil {
		toEvict.CloseWithCode(CloseReplaced, "replaced by newer connection")
	}
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
	if room, ok := h.byUser[c.UserID]; ok {
		delete(room, c)
		if len(room) == 0 {
			delete(h.byUser, c.UserID)
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

// Client — одно WS-соединение чата.
type Client struct {
	Hub         *Hub
	Conn        *websocket.Conn
	EventID     uuid.UUID
	UserID      uuid.UUID
	ConnectedAt time.Time

	send      chan []byte
	mu        sync.Mutex
	closed    bool
	closeCode int
}

func NewClient(hub *Hub, conn *websocket.Conn, eventID, userID uuid.UUID) *Client {
	return &Client{
		Hub:         hub,
		Conn:        conn,
		EventID:     eventID,
		UserID:      userID,
		ConnectedAt: time.Now(),
		send:        make(chan []byte, 32),
	}
}

// Enqueue кладёт сообщение в буфер. При переполнении закрывает клиента
// с кодом 1008 (policy violation) — это перегрузка канала, не LRU.
func (c *Client) Enqueue(data []byte) {
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

// CloseWithCode инициирует закрытие соединения с заданным close-code.
// Канал send закрывается; writer-loop в handler'е увидит это и отправит
// CloseMessage с этим кодом. Идемпотентно (повторный вызов — no-op).
func (c *Client) CloseWithCode(code int, _ string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	c.closeCode = code
	close(c.send)
}

func (c *Client) Send() <-chan []byte { return c.send }

// CloseCode отдаёт код, с которым нужно закрыть conn (0 — если не задан,
// например при естественном EOF: тогда writer закроет с 1000).
func (c *Client) CloseCode() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeCode
}
