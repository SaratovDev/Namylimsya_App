package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// После прошедших событий лимит «2 активных события» не должен блокировать
// создание новых — прошедшие не считаются.
func TestEventLimitExcludesPastEvents(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	email := uniqueEmail("elp")
	orgToken := env.registerAndLogin(t, email, "elp_"+shortID())

	orgUser, err := env.queries.GetUserByEmail(ctx, email)
	require.NoError(t, err)

	// Создаём 2 события — это ровно лимит.
	for i := 0; i < 2; i++ {
		r := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events", token: orgToken,
			body: map[string]any{
				"title": "Past", "category": "прогулки",
				"starts_at": futureTime(time.Duration(24+i) * time.Hour).Format(time.RFC3339),
				"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			},
		})
		require.Equal(t, 201, r.Status, r.Raw)
	}

	// Симулируем, что оба события уже прошли.
	_, err = env.pgPool.Exec(ctx,
		`UPDATE events SET starts_at = now() - interval '1 hour' WHERE organizer_id = $1`,
		orgUser.ID)
	require.NoError(t, err)

	// 3-е событие в будущем — должно пройти: прошедшие не занимают лимит.
	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Future", "category": "прогулки",
			"starts_at": futureTime(48 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)
}

// MarkPastEventsCompleted переводит прошедшие активные события в completed.
func TestMarkPastEventsCompleted(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	orgToken := env.registerAndLogin(t, uniqueEmail("mpc"), "mpc_"+shortID())

	// Создаём активное событие в будущем.
	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Will complete", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)
	eventID := r.Body["id"].(string)

	// Сдвигаем в прошлое.
	// MarkPastEventsCompleted переводит в completed всё старше 2 часов
	// (дефолтная длительность «идёт сейчас»). Используем 3 часа, чтобы
	// гарантированно попасть в «завершённые».
	_, err := env.pgPool.Exec(ctx,
		`UPDATE events SET starts_at = now() - interval '3 hours' WHERE id = $1`, eventID)
	require.NoError(t, err)

	// Дёргаем воркер-запрос вручную (без ожидания тикера).
	err = env.queries.MarkPastEventsCompleted(ctx)
	require.NoError(t, err)

	// Проверяем, что статус стал completed.
	var status string
	err = env.pgPool.QueryRow(ctx,
		`SELECT status FROM events WHERE id = $1`, eventID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "completed", status)
}
