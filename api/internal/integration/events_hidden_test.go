package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Скрытое событие недоступно по прямой ссылке и не принимает заявки
// ни от кого, кроме организатора/админа/модератора.
func TestHiddenEventAccess(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _ := setupAdmin(t, env)
	orgToken := env.registerAndLogin(t, uniqueEmail("hid_o"), "hido_"+shortID())
	userToken := env.registerAndLogin(t, uniqueEmail("hid_u"), "hidu_"+shortID())

	// Создаём событие.
	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Hidden target", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// До скрытия — доступно всем.
	preGuest := env.srv.do(t, httpReq{method: "GET", path: "/api/v1/events/" + eventID})
	require.Equal(t, 200, preGuest.Status, preGuest.Raw)

	// Скрываем админом.
	hide := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/events/" + eventID + "/hide",
		token: adminToken,
	})
	require.Equal(t, 200, hide.Status, hide.Raw)

	// Гость — 404.
	guest := env.srv.do(t, httpReq{method: "GET", path: "/api/v1/events/" + eventID})
	require.Equal(t, 404, guest.Status, guest.Raw)

	// Другой юзер — 404.
	stranger := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: userToken,
	})
	require.Equal(t, 404, stranger.Status, stranger.Raw)

	// Организатор — 200.
	owner := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: orgToken,
	})
	require.Equal(t, 200, owner.Status, owner.Raw)

	// Админ — 200.
	admin := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: adminToken,
	})
	require.Equal(t, 200, admin.Status, admin.Raw)

	// Другой юзер не может подать заявку на скрытое событие.
	apply := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications",
		token: userToken,
	})
	require.Equal(t, 400, apply.Status, apply.Raw)
	require.Contains(t, apply.Body["message"], "недоступно")
}

// После unhide событие снова доступно.
func TestUnhideRestoresAccess(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _ := setupAdmin(t, env)
	orgToken := env.registerAndLogin(t, uniqueEmail("unh_o"), "unho_"+shortID())
	userToken := env.registerAndLogin(t, uniqueEmail("unh_u"), "unhu_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Unhide test", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status)
	eventID := resp.Body["id"].(string)

	// Скрываем.
	env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/events/" + eventID + "/hide",
		token: adminToken,
	})
	// Юзер не видит.
	h := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: userToken,
	})
	require.Equal(t, 404, h.Status)

	// Показываем.
	env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/events/" + eventID + "/unhide",
		token: adminToken,
	})
	// Юзер снова видит.
	u := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: userToken,
	})
	require.Equal(t, 200, u.Status, u.Raw)
}
