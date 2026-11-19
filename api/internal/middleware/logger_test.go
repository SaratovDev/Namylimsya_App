package middleware

import (
	"bytes"
	"log"
	"net/http/httptest"
	"strings"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func TestScrubTokenInURI(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"/ws/events/abc", "/ws/events/abc"},
		{"/ws/events/abc?token=eyJhbGciOi.eyJzdWIiOi.sig", "/ws/events/abc?token=***"},
		{"/ws/events/abc?token=eyJ&other=1", "/ws/events/abc?token=***&other=1"},
		{"/ws/events/abc?other=1&token=eyJ", "/ws/events/abc?other=1&token=***"},
		{"/ws/notifications?token=eyJ", "/ws/notifications?token=***"},
		{"/api/v1/events?foo=bar", "/api/v1/events?foo=bar"}, // токена нет — не трогаем
	}
	for _, c := range cases {
		got := scrubTokenInURI(c.in)
		if got != c.out {
			t.Errorf("scrubTokenInURI(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}

// Полный цикл: formatter логирует скрабленный URI, но r.URL не портится.
func TestScrubLogFormatterMasksToken(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)

	f := &ScrubLogFormatter{Logger: logger}

	req := httptest.NewRequest("GET", "/ws/events/abc?token=SECRET_TOKEN_123&x=1", nil)

	// Новый log entry — запоминает URI для логирования.
	entry := f.NewLogEntry(req)

	// Проверяем, что r.URL в handler'е не тронут — WSHandler должен
	// увидеть настоящий токен.
	if got := req.URL.Query().Get("token"); got != "SECRET_TOKEN_123" {
		t.Fatalf("r.URL.Query() должен сохранить токен, got %q", got)
	}

	// Пишем фиктивный результат.
	entry.Write(200, 42, nil, 0, nil)

	out := buf.String()
	if strings.Contains(out, "SECRET_TOKEN_123") {
		t.Fatalf("log содержит токен: %q", out)
	}
	if !strings.Contains(out, "token=***") {
		t.Fatalf("log не содержит 'token=***': %q", out)
	}
}

// Проверяем, что используется именно ScrubLogFormatter (smoke — интерфейс
// LogFormatter реализован).
var _ chimw.LogFormatter = (*ScrubLogFormatter)(nil)
