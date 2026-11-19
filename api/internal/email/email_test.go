package email

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// silentServer поднимает TCP-листенер, который принимает соединение,
// но ничего не отвечает. Возвращает host, port и stop-функцию.
func silentServer(t *testing.T) (host, port string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				// Держим соединение, но не отвечаем на SMTP-приветствие.
				<-done
			}(conn)
		}
	}()

	h, p, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	return h, p, func() {
		_ = ln.Close()
	}
}

// Send должен вернуться быстро по таймауту, а не висеть.
func TestSendTimesOutOnSilentServer(t *testing.T) {
	host, port, stop := silentServer(t)
	defer stop()

	s := &Sender{
		host:    host,
		port:    port,
		from:    "noreply@test.local",
		timeout: 300 * time.Millisecond,
	}

	start := time.Now()
	err := s.Send(context.Background(), Message{
		To: "user@test.local", Subject: "test", Body: "<p>hi</p>",
	})
	elapsed := time.Since(start)

	require.Error(t, err, "Send должен вернуть ошибку таймаута")
	require.Less(t, elapsed, 2*time.Second, "Send должен отвалиться быстро")
}

// Если ctx истекает раньше s.timeout — уважаем ctx.
func TestSendRespectsContextDeadline(t *testing.T) {
	host, port, stop := silentServer(t)
	defer stop()

	s := &Sender{
		host:    host,
		port:    port,
		from:    "noreply@test.local",
		timeout: 10 * time.Second, // большой, чтобы сработал ctx
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := s.Send(ctx, Message{To: "user@test.local", Subject: "t", Body: "b"})
	require.Error(t, err)
	require.Less(t, time.Since(start), 1*time.Second)
}

// disabled → Send возвращает nil мгновенно, без сетевых вызовов.
func TestSendDisabledIsNoop(t *testing.T) {
	s := &Sender{disabled: true, timeout: time.Second}
	start := time.Now()
	err := s.Send(context.Background(), Message{To: "x@y.z", Subject: "s", Body: "b"})
	require.NoError(t, err)
	require.Less(t, time.Since(start), 50*time.Millisecond)
}
