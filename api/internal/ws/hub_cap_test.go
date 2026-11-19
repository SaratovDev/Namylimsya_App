package ws

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// LRU: 4-й клиент у одного юзера закрывает старейшее (4408), maps чистятся.
func TestHubPerUserCapLRU(t *testing.T) {
	h := NewHub(3)
	userID := uuid.New()
	eventID := uuid.New()

	c1 := &Client{Hub: h, EventID: eventID, UserID: userID,
		ConnectedAt: time.Now(), send: make(chan []byte, 4)}
	h.Register(c1)
	time.Sleep(2 * time.Millisecond)

	c2 := &Client{Hub: h, EventID: eventID, UserID: userID,
		ConnectedAt: time.Now(), send: make(chan []byte, 4)}
	h.Register(c2)
	time.Sleep(2 * time.Millisecond)

	c3 := &Client{Hub: h, EventID: eventID, UserID: userID,
		ConnectedAt: time.Now(), send: make(chan []byte, 4)}
	h.Register(c3)
	time.Sleep(2 * time.Millisecond)

	c4 := &Client{Hub: h, EventID: eventID, UserID: userID,
		ConnectedAt: time.Now(), send: make(chan []byte, 4)}
	h.Register(c4)

	// c1 закрыт с 4408.
	require.True(t, c1.closed, "c1 должен быть закрыт (LRU)")
	require.Equal(t, CloseReplaced, c1.CloseCode())

	// c2, c3, c4 — живы.
	require.False(t, c2.closed)
	require.False(t, c3.closed)
	require.False(t, c4.closed)

	// byUser содержит 3 клиента (c2, c3, c4).
	h.mu.RLock()
	require.Len(t, h.byUser[userID], 3)
	h.mu.RUnlock()
}

// maxPerUser=0 — без ограничений.
func TestHubUnlimited(t *testing.T) {
	h := NewHub(0)
	userID := uuid.New()
	for i := 0; i < 10; i++ {
		c := &Client{Hub: h, EventID: uuid.New(), UserID: userID,
			ConnectedAt: time.Now(), send: make(chan []byte, 4)}
		h.Register(c)
		require.False(t, c.closed)
	}
}

// Тот же LRU для NotificationHub.
func TestNotificationHubPerUserCapLRU(t *testing.T) {
	h := NewNotificationHub(3)
	userID := uuid.New()

	c1 := &NotificationClient{Hub: h, UserID: userID,
		ConnectedAt: time.Now(), send: make(chan []byte, 4)}
	h.Register(c1)
	time.Sleep(2 * time.Millisecond)

	c2 := &NotificationClient{Hub: h, UserID: userID,
		ConnectedAt: time.Now(), send: make(chan []byte, 4)}
	h.Register(c2)
	time.Sleep(2 * time.Millisecond)

	c3 := &NotificationClient{Hub: h, UserID: userID,
		ConnectedAt: time.Now(), send: make(chan []byte, 4)}
	h.Register(c3)
	time.Sleep(2 * time.Millisecond)

	c4 := &NotificationClient{Hub: h, UserID: userID,
		ConnectedAt: time.Now(), send: make(chan []byte, 4)}
	h.Register(c4)

	require.True(t, c1.closed)
	require.Equal(t, CloseReplaced, c1.CloseCode())
	require.False(t, c2.closed)
	require.False(t, c3.closed)
	require.False(t, c4.closed)
}
