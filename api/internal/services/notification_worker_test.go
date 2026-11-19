package services

import (
	"context"
	"testing"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/email"

	"github.com/google/uuid"
)

// Start+Stop не должны ни паниковать, ни зависать:
// Start сам делает wg.Add(1), Stop вызывает wg.Wait().
// Пустые db/mailer допустимы — воркер их трогает только при flush непустого батча.
func TestNotificationWorker_StartStop(t *testing.T) {
	w := NewNotificationWorker(nil, nil, "")

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	// Небольшая пауза, чтобы Start успел встать в select.
	time.Sleep(50 * time.Millisecond)

	cancel()

	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
		// ОК
	case <-time.After(2 * time.Second):
		t.Fatal("NotificationWorker.Stop hung — Start/Stop wg рассинхронизированы")
	}
}

// Enqueue до Start не должен паниковать (канал буферизован, ёмкость 100).
func TestNotificationWorker_EnqueueWithoutStart(t *testing.T) {
	w := NewNotificationWorker(nil, nil, "")
	// Просто проверяем, что вызов не блокируется навсегда при одном элементе.
	done := make(chan struct{})
	go func() {
		w.Enqueue([16]byte{}, "test@example.com", "test", map[string]any{})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Enqueue blocked on empty worker queue")
	}
}

// Enqueue не блокируется, когда канал полон. Иначе Apply/Approve/Cancel
// висели бы при зависшем SMTP.
func TestNotificationWorker_EnqueueDoesNotBlockWhenFull(t *testing.T) {
	w := NewNotificationWorker(nil, nil, "")
	// Start не вызываем — канал не вычитывается. Заливаем больше, чем
	// буфер (100), и проверяем, что вызов не блокируется.

	done := make(chan struct{})
	go func() {
		for i := 0; i < 150; i++ {
			w.Enqueue(uuid.Nil, "u@example.com", "test", nil)
		}
		close(done)
	}()

	select {
	case <-done:
		// Ок — Enqueue дропает лишние, не блокируется.
	case <-time.After(2 * time.Second):
		t.Fatal("Enqueue blocked on full queue — должен дропать и возвращаться")
	}
}

// TestNotificationWorker_FlushesOnShutdown проверяет, что при shutdown
// воркер НЕ зависает и flush завершается — даже если ctx уже отменён.
// Без фикса (flushCtx в case <-ctx.Done()) Stop() зависал бы, потому что
// w.flush(ctx, ...) с отменённым ctx → smtp.Send падает с context.Canceled.

func TestNotificationWorker_FlushesOnShutdown_X(t *testing.T) {
	// Mailer в disabled-режиме: Send возвращает nil мгновенно, без SMTP.
	// Безопасно для юнит-теста — проверяем, что flush НЕ паникует и не виснет.
	mailer := email.NewSender("localhost", "1025", "", "", "noreply@test.local", true)

	w := NewNotificationWorker(nil, mailer, "")
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	// Кладём письмо в очередь.
	w.Enqueue(uuid.New(), "test@example.com", "test", map[string]any{})

	// Даём 50ms встать в select (batch получит письмо).
	time.Sleep(50 * time.Millisecond)

	// Отменяем ctx → Start ловит ctx.Done() → flush с flushCtx → return.
	cancel()

	// Stop ждёт wg.Wait(). Если flush завис — Stop зависнет.
	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Ок — воркер завершился, flush выполнился (не завис).
	case <-time.After(15 * time.Second):
		t.Fatal("NotificationWorker.Stop hung — flush с flushCtx не завершился")
	}
}
