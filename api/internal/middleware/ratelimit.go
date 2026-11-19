package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/httpx"
	"github.com/SaratovDev/Namylimsya_App/api/internal/ratelimit"
	"github.com/rs/zerolog/log"
)

// IPRateLimit — ограничивает количество запросов с одного IP.
// Ключ: prefix + IP. rl.Allow — sliding window по Redis.
//
// failOpen — что делать при ошибке Redis (недоступен/таймаут):
//   - true  — пропустить запрос (лента, события: лучше обслужить, чем упасть);
//   - false — заблокировать 429 (auth: лучше не пустить, чем пустить брутфорс).
//
// Для /auth/* ВСЕГДА failOpen=false. Для остального — true.
func IPRateLimit(rl *ratelimit.Limiter, prefix string, limit int, window time.Duration, failOpen bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if ip == "" {
				ip = "unknown"
			}
			key := prefix + ip
			allowed, err := rl.Allow(r.Context(), key, limit, window)
			if err != nil {
				log.Warn().Err(err).Str("key", key).Bool("fail_open", failOpen).
					Msg("ratelimit: redis unavailable")
				if failOpen {
					// Лента, события — обслужим без лимита.
					next.ServeHTTP(w, r)
					return
				}
				// /auth — блокируем: лучше 429, чем неограниченный брутфорс.
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				httpx.Error(w, http.StatusTooManyRequests, "rate_limit", "service temporarily unavailable, try again later")
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				httpx.Error(w, http.StatusTooManyRequests, "rate_limit", "too many requests, try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP берёт IP из r.RemoteAddr (middleware.RealIP уже перезаписал его из XFF).
// IPv6-safe: SplitHostPort корректно работает с "[2001:db8::1]:443" и "2001:db8::1".
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// После RealIP RemoteAddr уже без порта ("1.2.3.4", "2001:db8::1").
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
