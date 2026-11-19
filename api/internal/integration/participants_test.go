package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Гость видит только approved-участников; org первый.
func TestParticipantsApprovedOnly(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("po_org"), "poo_"+shortID())
	u1Token := env.registerAndLogin(t, uniqueEmail("po_u1"), "pou1_"+shortID())
	u2Token := env.registerAndLogin(t, uniqueEmail("po_u2"), "pou2_"+shortID())
	u3Token := env.registerAndLogin(t, uniqueEmail("po_u3"), "pou3_"+shortID())

	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Participants", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)
	eventID := r.Body["id"].(string)

	// u1 → approve, u2 → pending, u3 → reject.
	a1 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: u1Token,
	})
	require.Equal(t, 201, a1.Status, a1.Raw)
	env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + a1.Body["id"].(string) + "/approve",
		token: orgToken,
	})

	a2 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: u2Token,
	})
	require.Equal(t, 201, a2.Status, a2.Raw)

	a3 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: u3Token,
	})
	require.Equal(t, 201, a3.Status, a3.Raw)
	env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + a3.Body["id"].(string) + "/reject",
		token: orgToken,
	})

	// Гость — видит только approved (u1), орг первым? Нет, орг не подаёт заявку.
	// Значит, орг не в списке approved-заявок. Плюс u1.
	resp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/participants",
	})
	require.Equal(t, 200, resp.Status, resp.Raw)
	parts := resp.Body["participants"].([]any)
	require.Len(t, parts, 1, "только approved (u1), pending и rejected скрыты")
	require.Equal(t, "po_u1", parts[0].(map[string]any)["username"])
}

// Публичный эндпоинт без авторизации.
func TestParticipantsPublic(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("pp_o"), "ppo_"+shortID())
	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Pub", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status)
	eventID := r.Body["id"].(string)

	resp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/participants",
	})
	require.Equal(t, 200, resp.Status, resp.Raw)
	require.Empty(t, resp.Body["participants"].([]any))
}

// show_age=false → нет поля age.
func TestParticipantsAgeHidden(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	orgToken := env.registerAndLogin(t, uniqueEmail("pa_o"), "pao_"+shortID())
	appEmail := uniqueEmail("pa_u")
	appToken := env.registerAndLogin(t, appEmail, "pau_"+shortID())

	// Установить birth_date и show_age=false напрямую (UI не даёт это
	// при регистрации).
	appUser, _ := env.queries.GetUserByEmail(ctx, appEmail)
	_, err := env.pgPool.Exec(ctx,
		`UPDATE users SET birth_date = '1990-01-15', show_age = false WHERE id = $1`,
		appUser.ID)
	require.NoError(t, err)

	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Age hidden", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status)
	eventID := r.Body["id"].(string)

	a := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: appToken,
	})
	require.Equal(t, 201, a.Status)
	env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + a.Body["id"].(string) + "/approve",
		token: orgToken,
	})

	resp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/participants",
	})
	parts := resp.Body["participants"].([]any)
	require.Len(t, parts, 1)
	p := parts[0].(map[string]any)
	require.NotContains(t, p, "age", "show_age=false → age не возвращается")
}
