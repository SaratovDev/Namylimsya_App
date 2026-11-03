package middleware

import (
	"net/http"

	"github.com/SaratovDev/Namylimsya_App/api/internal/httpx"
)

// RequireRole — пропускает только пользователей с ролью из списка.
// Должен идти ПОСЛЕ AuthRequired (нужно, чтобы в context был role).
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := r.Context().Value(RoleKey).(string)
			if !ok || !allowed[role] {
				httpx.Error(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
