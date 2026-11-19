package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMyApplications(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("mine_o"), "mo2_"+shortID())
	org2Token := env.registerAndLogin(t, uniqueEmail("mine_o2"), "mo3_"+shortID())
	userToken := env.registerAndLogin(t, uniqueEmail("mine_u"), "mu_"+shortID())

	// 2 события от разных органов.
	e1 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "App1", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, e1.Status)
	event1 := e1.Body["id"].(string)

	e2 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: org2Token,
		body: map[string]any{
			"title": "App2", "category": "спорт",
			"starts_at": futureTime(25 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, e2.Status)
	event2 := e2.Body["id"].(string)

	// Подаём 2 заявки.
	app1 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + event1 + "/applications", token: userToken,
	})
	require.Equal(t, 201, app1.Status)
	app2 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + event2 + "/applications", token: userToken,
	})
	require.Equal(t, 201, app2.Status)

	// Одобряем первую, вторую оставляем pending.
	env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + app1.Body["id"].(string) + "/approve", token: orgToken,
	})

	// GET /users/me/applications — 2 записи с инфой о событии.
	mineResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me/applications", token: userToken,
	})
	require.Equal(t, 200, mineResp.Status, mineResp.Raw)
	apps := mineResp.Body["applications"].([]any)
	require.Len(t, apps, 2)

	byStatus := map[string]map[string]any{}
	for _, a := range apps {
		m := a.(map[string]any)
		byStatus[m["status"].(string)] = m
	}
	require.Contains(t, byStatus, "approved")
	require.Contains(t, byStatus, "pending")

	// Каждая заявка содержит поля события.
	for _, a := range apps {
		m := a.(map[string]any)
		require.NotEmpty(t, m["event_title"])
		require.NotEmpty(t, m["event_starts_at"])
		require.NotEmpty(t, m["organizer_name"])
		require.Equal(t, "Саратов", m["event_city"])
	}

	// Гость — 401.
	guestResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me/applications",
	})
	require.Equal(t, 401, guestResp.Status)
}
