package middleware

import (
	"net"
	"net/http"
	"strings"
)

// RealIP заменяет chi's middleware.RealIP на версию, которая доверяет
// X-Forwarded-For только при:
//  1. trustedProxy = true (явно за reverse-proxy);
//  2. peer (r.RemoteAddr) — private/loopback IP (Docker bridge, Caddy).
func RealIP(trustedProxy bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if trustedProxy && isPrivatePeer(r.RemoteAddr) {
				if ip := realIP(r); ip != "" {
					r.RemoteAddr = ip
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isPrivatePeer — true, если r.RemoteAddr (host:port) — loopback, private
// (RFC1918 / link-local). Docker bridge (172.16.0.0/12) попадает сюда —
// это peer от Caddy.
func isPrivatePeer(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

// realIP возвращает клиентский IP из X-Forwarded-For.
//
// Правила:
//   - читаем ТОЛЬКО X-Forwarded-For. X-Real-IP / True-Client-IP не читаем
//     вовсе: Caddy их не выставляет и не вычищает → это прямая инъекция
//     от клиента (bypass rate-limit одним заголовком);
//   - берём ПОСЛЕДНИЙ (правый) валидный IP: при одном proxy-хопе его
//     дописал ближайший trusted proxy (Caddy). Всё левее — client-supplied:
//     Caddy по умолчанию APPEND'ит XFF, а не заменяет, поэтому первый
//     элемент цепочки контролирует атакующий.
//
// ИНВАРИАНТ: ровно один proxy-хоп, и в Caddyfile стоит
//
//	header_up X-Forwarded-For {remote_host}
//
// Если появится второй хоп (CDN перед Caddy) — нужно идти справа налево,
// пропуская IP доверенных прокси, а не брать просто правый.
func realIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return ""
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(parts[i]))
		if ip != nil {
			return ip.String()
		}
	}
	return ""
}
