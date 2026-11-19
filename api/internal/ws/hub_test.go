package ws

import (
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestHub_EnqueueAfterClose_NoPanic(t *testing.T) {
	hub := NewHub(0)
	c := &Client{
		Hub:     hub,
		EventID: uuid.New(),
		UserID:  uuid.New(),
		send:    make(chan []byte, 1),
	}

	// Заполняем канал.
	c.Enqueue([]byte("1"))

	// Переполняем — Enqueue закроет канал.
	c.Enqueue([]byte("2"))

	// Многократно дёргаем Enqueue параллельно — не должно быть panic.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Enqueue([]byte("x"))
		}()
	}
	wg.Wait()

	// После close канал закрыт.
	select {
	case _, ok := <-c.Send():
		if ok {
			// Первое сообщение могло остаться в буфере — это ок.
		}
	default:
		// ничего
	}
}

func TestNotificationHub_EnqueueAfterClose_NoPanic(t *testing.T) {
	hub := NewNotificationHub(0)
	c := &NotificationClient{
		Hub:    hub,
		UserID: uuid.New(),
		send:   make(chan []byte, 1),
	}

	c.Enqueue([]byte("1"))
	c.Enqueue([]byte("2"))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Enqueue([]byte("x"))
		}()
	}
	wg.Wait()
}
