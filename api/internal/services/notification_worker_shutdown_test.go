package services

import (
	"context"
	"testing"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/email"
	"github.com/google/uuid"
)

// TestNotificationWorker_FlushesOnShutdown проверяет, что при shutdown
// воркер НЕ зависает и flush завершается — даже если ctx уже отменён.
// Без фикса (flushCtx в case <-ctx.Done()) Stop() зависал бы навсегда
// (mailer.Send с отменённым ctx падал бы мгновенно, но если бы SMTP висел —
// wg.Wait() ждал бы бесконечно).
func TestNotificationWorker_FlushesOnShutdown(t *testing.T) {
	// Mailer в disabled-режиме: Send возвращает nil мгновенно, без SMTP.
	// Это безопасно для юнит-теста — проверяем, что flush НЕ паникует
	// на mailer и что Stop() завершается за таймаут.
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
