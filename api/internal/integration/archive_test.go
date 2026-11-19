package integration

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Роут /events/archive доступен анонимно и возвращает 200.
func TestArchiveRoutePublic(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	r := env.srv.do(t, httpReq{method: "GET", path: "/api/v1/events/archive"})
	require.Equal(t, http.StatusOK, r.Status, r.Raw)
	require.NotNil(t, r.Body["events"], "поле events должно присутствовать")
}

// Завершённое событие видно в архиве всем.
func TestArchiveShowsCompleted(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	orgToken := env.registerAndLogin(t, uniqueEmail("arc_o"), "arco_"+shortID())
	uniqueCity := "Саратов-" + shortID()

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Archived", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// Сдвигаем в прошлое и переводим в completed (эмуляция EventWorker).
	_, err := env.pgPool.Exec(ctx,
		`UPDATE events SET starts_at = now() - interval '3 hours', status = 'completed' WHERE id = $1`,
		eventID)
	require.NoError(t, err)

	// Гость видит.
	r := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/archive?city=" + url.QueryEscape(uniqueCity),
	})
	require.Equal(t, 200, r.Status, r.Raw)
	events := r.Body["events"].([]any)
	require.Len(t, events, 1)
	require.Equal(t, "completed", events[0].(map[string]any)["status"])
}

// Активное событие «идёт сейчас» — видно в архиве с тем же статусом active.
func TestArchiveShowsOngoing(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	orgToken := env.registerAndLogin(t, uniqueEmail("aong_o"), "aongo_"+shortID())
	uniqueCity := "Саратов-" + shortID()

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Ongoing", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// Стартовало 30 минут назад, ещё active.
	_, err := env.pgPool.Exec(ctx,
		`UPDATE events SET starts_at = now() - interval '30 minutes' WHERE id = $1`, eventID)
	require.NoError(t, err)

	r := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/archive?city=" + url.QueryEscape(uniqueCity),
	})
	require.Equal(t, 200, r.Status, r.Raw)
	events := r.Body["events"].([]any)
	require.Len(t, events, 1, "«идёт сейчас» должно быть в архиве")
	require.Equal(t, "active", events[0].(map[string]any)["status"])
}

// Активное событие старше 2 часов не показывается (должно быть completed
// через EventWorker; если он не сработал — архив всё равно его скрывает,
// чтобы не путать).
func TestArchiveHidesStaleActive(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	orgToken := env.registerAndLogin(t, uniqueEmail("astale_o"), "astaleo_"+shortID())
	uniqueCity := "Саратов-" + shortID()

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Stale", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// Стартовало 3 часа назад, всё ещё active (воркер не успел).
	_, err := env.pgPool.Exec(ctx,
		`UPDATE events SET starts_at = now() - interval '3 hours' WHERE id = $1`, eventID)
	require.NoError(t, err)

	r := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/archive?city=" + url.QueryEscape(uniqueCity),
	})
	require.Equal(t, 200, r.Status, r.Raw)
	require.Empty(t, r.Body["events"].([]any),
		"«зависшее» active старше 2 часов не должно быть в архиве")
}

// Отменённое событие видно только тем, кто подал заявку (approved или pending).
func TestArchiveCancelledVisibility(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("acan_o"), "acano_"+shortID())
	appliedToken := env.registerAndLogin(t, uniqueEmail("acan_a"), "acana_"+shortID())
	strangerToken := env.registerAndLogin(t, uniqueEmail("acan_s"), "acans_"+shortID())
	uniqueCity := "Саратов-" + shortID()

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Cancelled archive", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// Подаём заявку (pending — по Q1 тоже даёт видимость cancelled).
	apply := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: appliedToken,
	})
	require.Equal(t, 201, apply.Status, apply.Raw)

	// Отменяем.
	cancel := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/cancel", token: orgToken,
		body: map[string]any{"reason": "плохая погода"},
	})
	require.Equal(t, 200, cancel.Status, cancel.Raw)

	// Гость — не видит.
	guest := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/archive?city=" + url.QueryEscape(uniqueCity),
	})
	require.Equal(t, 200, guest.Status)
	require.Empty(t, guest.Body["events"].([]any))

	// Сторонний (без заявки) — не видит.
	stranger := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/archive?city=" + url.QueryEscape(uniqueCity),
		token: strangerToken,
	})
	require.Empty(t, stranger.Body["events"].([]any))

	// Участник с pending — видит.
	applied := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/archive?city=" + url.QueryEscape(uniqueCity),
		token: appliedToken,
	})
	events := applied.Body["events"].([]any)
	require.Len(t, events, 1, "pending-участник видит отменённое событие")
	require.Equal(t, "cancelled", events[0].(map[string]any)["status"])

	// Организатор — видит.
	org := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/archive?city=" + url.QueryEscape(uniqueCity),
		token: orgToken,
	})
	require.Len(t, org.Body["events"].([]any), 1)
}

// Фильтры работают.
func TestArchiveFilters(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	orgToken := env.registerAndLogin(t, uniqueEmail("af_o"), "afo_"+shortID())
	uniqueCity := "Саратов-" + shortID()

	// Создаём 2 события разных категорий.
	for i, cat := range []string{"прогулки", "спорт"} {
		r := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events", token: orgToken,
			body: map[string]any{
				"title": "F " + cat, "category": cat,
				"starts_at": futureTime(time.Duration(24+i) * time.Hour).Format(time.RFC3339),
				"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
			},
		})
		require.Equal(t, 201, r.Status, r.Raw)
	}

	// Переводим оба в completed.
	_, err := env.pgPool.Exec(ctx,
		`UPDATE events SET status='completed', starts_at = now() - interval '1 day' WHERE city = $1`,
		uniqueCity)
	require.NoError(t, err)

	// Фильтр по категории "спорт" — 1 событие.
	r := env.srv.do(t, httpReq{
		method: "GET",
		path:   "/api/v1/events/archive?city=" + url.QueryEscape(uniqueCity) + "&category=спорт",
	})
	require.Equal(t, 200, r.Status, r.Raw)
	events := r.Body["events"].([]any)
	require.Len(t, events, 1)
	require.Equal(t, "спорт", events[0].(map[string]any)["category"])

	// Фильтр по date_from в будущем — пусто.
	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	r2 := env.srv.do(t, httpReq{
		method: "GET",
		path: "/api/v1/events/archive?city=" + url.QueryEscape(uniqueCity) +
			"&date_from=" + url.QueryEscape(future),
	})
	require.Equal(t, 200, r2.Status)
	require.Empty(t, r2.Body["events"].([]any))
}

// Скрытое модератором событие не всплывает в публичном архиве после завершения.
func TestArchiveHidesHiddenEvent(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	adminToken, _ := setupAdmin(t, env)
	orgToken := env.registerAndLogin(t, uniqueEmail("ahid_o"), "ahido_"+shortID())
	uniqueCity := "Саратов-" + shortID()

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Hidden archive", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// Завершаем (эмуляция EventWorker) и скрываем админом.
	_, err := env.pgPool.Exec(ctx,
		`UPDATE events SET starts_at = now() - interval '3 hours', status = 'completed' WHERE id = $1`,
		eventID)
	require.NoError(t, err)
	hide := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/events/" + eventID + "/hide", token: adminToken,
	})
	require.Equal(t, 200, hide.Status, hide.Raw)

	// Гость в архиве не видит скрытое.
	guest := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/archive?city=" + url.QueryEscape(uniqueCity),
	})
	require.Equal(t, 200, guest.Status, guest.Raw)
	require.Empty(t, guest.Body["events"].([]any),
		"скрытое событие не должно быть в публичном архиве")

	// Организатор видит своё скрытое в архиве.
	owner := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/archive?city=" + url.QueryEscape(uniqueCity),
		token:  orgToken,
	})
	require.Equal(t, 200, owner.Status, owner.Raw)
	require.Len(t, owner.Body["events"].([]any), 1,
		"организатор видит своё скрытое событие в архиве")
}
