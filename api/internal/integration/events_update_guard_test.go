package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Создаёт событие через API и возвращает (eventID, orgToken).
func createEventForUpdate(t *testing.T, env *testEnv) (string, string) {
	t.Helper()
	orgToken := env.registerAndLogin(t, uniqueEmail("upd_o"), "updo_"+shortID())
	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Update target", "category": "прогулки",
			"starts_at": futureTime(48 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)
	return r.Body["id"].(string), orgToken
}

// Нельзя поставить starts_at в прошлое.
func TestUpdateEventRejectsPastStartsAt(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	eventID, orgToken := createEventForUpdate(t, env)

	past := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	r := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/events/" + eventID, token: orgToken,
		body: map[string]any{
			"title": "Update target", "category": "прогулки",
			"starts_at": past,
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, http.StatusBadRequest, r.Status, r.Raw)
	require.Contains(t, r.Body["message"], "будущем")
}

// Нельзя редактировать отменённое событие.
func TestUpdateEventRejectsCancelled(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	eventID, orgToken := createEventForUpdate(t, env)

	// Отменяем.
	cancel := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/cancel", token: orgToken,
		body: map[string]any{"reason": "test"},
	})
	require.Equal(t, 200, cancel.Status, cancel.Raw)

	// Пытаемся редактировать — 403.
	r := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/events/" + eventID, token: orgToken,
		body: map[string]any{
			"title": "Updated", "category": "спорт",
			"starts_at": futureTime(72 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, http.StatusForbidden, r.Status, r.Raw)
	require.Contains(t, r.Body["message"], "активное")
}

// participant_limit ниже числа approved — нельзя.
func TestUpdateEventRejectsLimitBelowApproved(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	eventID, orgToken := createEventForUpdate(t, env)

	// Два участника подают и одобряются.
	for i := 0; i < 2; i++ {
		appToken := env.registerAndLogin(t, uniqueEmail("upd_app"), "upda_"+shortID())
		app := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: appToken,
		})
		require.Equal(t, 201, app.Status, app.Raw)
		appID := app.Body["id"].(string)
		ap := env.srv.do(t, httpReq{
			method: "PATCH", path: "/api/v1/applications/" + appID + "/approve", token: orgToken,
		})
		require.Equal(t, 200, ap.Status, ap.Raw)
	}

	// Проверяем, что approved = 2.
	var approved int
	err := env.pgPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM applications WHERE event_id=$1 AND status='approved'`, eventID).
		Scan(&approved)
	require.NoError(t, err)
	require.Equal(t, 2, approved)

	// Пытаемся выставить лимит 1 — 400.
	limit := int32(1)
	r := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/events/" + eventID, token: orgToken,
		body: map[string]any{
			"title": "Update target", "category": "прогулки",
			"starts_at": futureTime(48 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			"participant_limit": limit,
		},
	})
	require.Equal(t, http.StatusBadRequest, r.Status, r.Raw)
	require.Contains(t, r.Body["message"], "одобренных")

	// Лимит 2 — 200.
	limit2 := int32(2)
	r2 := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/events/" + eventID, token: orgToken,
		body: map[string]any{
			"title": "Update target", "category": "прогулки",
			"starts_at": futureTime(48 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			"participant_limit": limit2,
		},
	})
	require.Equal(t, 200, r2.Status, r2.Raw)
}

// Нормальное редактирование активного события — 200.
func TestUpdateEventHappyPath(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	eventID, orgToken := createEventForUpdate(t, env)

	r := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/events/" + eventID, token: orgToken,
		body: map[string]any{
			"title": "Updated title", "category": "спорт",
			"description": "new desc",
			"starts_at":   futureTime(72 * time.Hour).Format(time.RFC3339),
			"city":        "Саратов", "lat": 51.54, "lng": 46.04,
		},
	})
	require.Equal(t, 200, r.Status, r.Raw)
	require.Equal(t, "Updated title", r.Body["title"])
	require.Equal(t, "спорт", r.Body["category"])
}
