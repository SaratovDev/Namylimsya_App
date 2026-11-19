package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// ScrubLogFormatter — замена chi's DefaultLogFormatter, которая маскирует
// значение query-параметра token в RequestURI при логировании. Иначе
// WS-handshake (?token=<jwt>) пишет полный access-токен в stdout и логи
// reverse-proxy, что даёт 15-минутный window для угона сессии любым, у
// кого есть доступ к логам.
//
// r.URL не трогается — handler (WSHandler.Handle) по-прежнему читает токен
// через r.URL.Query().Get("token"). Скрабится только строка для лога.
type ScrubLogFormatter struct {
	Logger chimw.LoggerInterface
}

func (f *ScrubLogFormatter) NewLogEntry(r *http.Request) chimw.LogEntry {
	uri := r.RequestURI
	if strings.Contains(uri, "token=") {
		uri = scrubTokenInURI(uri)
	}
	return &scrubLogEntry{
		logger: f.Logger,
		method: r.Method,
		uri:    uri,
		proto:  r.Proto,
	}
}

type scrubLogEntry struct {
	logger chimw.LoggerInterface
	method string
	uri    string
	proto  string
}

func (e *scrubLogEntry) Write(status, bytes int, header http.Header, elapsed time.Duration, extra interface{}) {
	e.logger.Print(fmt.Sprintf("%s %s %s - %d %dB in %s",
		e.method, e.uri, e.proto, status, bytes, elapsed))
}

func (e *scrubLogEntry) Panic(v interface{}, stack []byte) {
	e.logger.Print(fmt.Sprintf("panic: %v", v))
}

// scrubTokenInURI заменяет значение token=... на "token=***".
// Работает и с токеном в середине query, и с токеном в конце.
// Пример: /ws/events/abc?token=eyJ...&x=1 → /ws/events/abc?token=***&x=1
func scrubTokenInURI(uri string) string {
	path := uri
	q := ""
	if i := strings.Index(uri, "?"); i != -1 {
		path = uri[:i]
		q = uri[i+1:]
	}
	if q == "" {
		return uri
	}
	parts := strings.Split(q, "&")
	for i, p := range parts {
		if strings.HasPrefix(p, "token=") {
			parts[i] = "token=***"
		}
	}
	return path + "?" + strings.Join(parts, "&")
}
