package integration

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Pending больше не имеет доступа к чату.
// Pending имеет доступ к чату — люди знакомятся до одобрения.
func TestChatAccessPendingAllowed(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("ca_o"), "cao_"+shortID())
	pendToken := env.registerAndLogin(t, uniqueEmail("ca_pend"), "cap_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "CA pending", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status)
	eventID := resp.Body["id"].(string)

	// Pending подаёт заявку.
	env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: pendToken,
	})

	// Pending может читать историю (200, даже если пусто).
	histResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: pendToken,
	})
	require.Equal(t, 200, histResp.Status, "pending must read chat: %s", histResp.Raw)

	// Pending может отправлять.
	sendResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/messages", token: pendToken,
		body: map[string]any{"text": "hi from pending"},
	})
	require.Equal(t, 201, sendResp.Status, "pending must send chat: %s", sendResp.Raw)

	// Организатор видит сообщение pending в истории.
	orgHist := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: orgToken,
	})
	require.Equal(t, 200, orgHist.Status, orgHist.Raw)
	msgs := orgHist.Body["messages"].([]any)
	require.Len(t, msgs, 1)
	require.Equal(t, "hi from pending", msgs[0].(map[string]any)["text"])

	// Сторонний (без заявки) — 403.
	strangerToken := env.registerAndLogin(t, uniqueEmail("ca_str"), "cas_"+shortID())
	strangerHist := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: strangerToken,
	})
	require.Equal(t, 403, strangerHist.Status, "stranger must not read chat")
}

// Cancelled событие — чат закрыт для всех, включая организатора.
func TestChatAccessCancelledClosed(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("cc_o"), "cco_"+shortID())
	appToken := env.registerAndLogin(t, uniqueEmail("cc_app"), "cca_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "CA cancelled", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status)
	eventID := resp.Body["id"].(string)

	// Одобренный участник.
	appResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: appToken,
	})
	require.Equal(t, 201, appResp.Status)
	appID := appResp.Body["id"].(string)
	env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID + "/approve", token: orgToken,
	})

	// До отмены — чат работает.
	beforeResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: appToken,
	})
	require.Equal(t, 200, beforeResp.Status)

	// Отменяем.
	cancelResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/cancel", token: orgToken,
		body: map[string]any{"reason": "test"},
	})
	require.Equal(t, 200, cancelResp.Status)

	// После отмены — 403 для участника и органа.
	for _, tok := range []string{orgToken, appToken} {
		r := env.srv.do(t, httpReq{
			method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: tok,
		})
		require.Equal(t, 403, r.Status, "cancelled event chat must be closed: %s", r.Raw)
	}
}

// Cancel не своего события — 403; Cancel уже отменённого — 400.
func TestCancelEventErrors(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("ce_o"), "ceo_"+shortID())
	strangerToken := env.registerAndLogin(t, uniqueEmail("ce_s"), "ces_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "CE", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status)
	eventID := resp.Body["id"].(string)

	// Не своё — 403.
	notOwner := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/cancel", token: strangerToken,
		body: map[string]any{"reason": "x"},
	})
	require.Equal(t, 403, notOwner.Status, notOwner.Raw)

	// Отменяем первый раз — 200.
	first := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/cancel", token: orgToken,
		body: map[string]any{"reason": "test"},
	})
	require.Equal(t, 200, first.Status, first.Raw)

	// Повторно — 400 (уже отменено).
	second := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/cancel", token: orgToken,
		body: map[string]any{"reason": "test"},
	})
	require.Equal(t, 400, second.Status, second.Raw)
	require.Contains(t, second.Body["message"], "не активно")

	_ = url.QueryEscape // защита от unused import, если выше не используется
}
