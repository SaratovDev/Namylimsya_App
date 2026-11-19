package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Скрытое админом событие — чат закрыт для всех, включая организатора.
func TestChatAccessHiddenClosed(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _ := setupAdmin(t, env)
	orgToken := env.registerAndLogin(t, uniqueEmail("chh_o"), "chho_"+shortID())
	appToken := env.registerAndLogin(t, uniqueEmail("chh_a"), "chha_"+shortID())

	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Chat hidden", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)
	eventID := r.Body["id"].(string)

	// Одобренный участник.
	apply := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: appToken,
	})
	require.Equal(t, 201, apply.Status, apply.Raw)
	appID := apply.Body["id"].(string)
	env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID + "/approve", token: orgToken,
	})

	// До скрытия — чат работает.
	before := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: appToken,
	})
	require.Equal(t, 200, before.Status, before.Raw)

	// Админ скрывает событие.
	hide := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/events/" + eventID + "/hide", token: adminToken,
	})
	require.Equal(t, 200, hide.Status, hide.Raw)

	// После скрытия — 403 и для организатора, и для участника.
	for _, tok := range []string{orgToken, appToken} {
		h := env.srv.do(t, httpReq{
			method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: tok,
		})
		require.Equal(t, http.StatusForbidden, h.Status,
			"hidden event chat must be closed: %s", h.Raw)
	}
}

// D4: при отсутствии доступа — 403 без утечки err.Error();
// при ошибке БД — 500 generic (InternalError), а не 403.
// Второе проверить без остановки БД сложно, поэтому проверяем 403 без деталей SQL.
func TestChatHistoryForbiddenNoLeak(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("chl_o"), "chlo_"+shortID())
	strangerToken := env.registerAndLogin(t, uniqueEmail("chl_s"), "chls_"+shortID())

	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Chat noleak", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)
	eventID := r.Body["id"].(string)

	// Сторонний — 403, без внутренних деталей.
	h := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: strangerToken,
	})
	require.Equal(t, http.StatusForbidden, h.Status)
	msg, _ := h.Body["message"].(string)
	require.NotContains(t, msg, "sql", "не должно быть SQL-деталей в 403")
	require.NotContains(t, msg, "pgx", "не должно быть pgx-деталей в 403")
	require.Equal(t, "you are not a member of this chat", msg,
		"403 должен содержать фиксированный текст, не err.Error()")
}
