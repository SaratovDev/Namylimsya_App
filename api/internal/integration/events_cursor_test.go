package integration

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Пагинация не пропускает и не дублирует события с одинаковым starts_at.
func TestFeedCursorTiesDontDropEvents(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	uniqueCity := "Саратов-" + shortID()

	// 5 событий от 5 разных организаторов — у каждого лимит 2 активных,
	// 1 активное проходит без проблем. Один орг упёрся бы в лимит на 3-м.
	ids := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		orgToken := env.registerAndLogin(t, uniqueEmail("cur_o"), "curo_"+shortID())
		r := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events", token: orgToken,
			body: map[string]any{
				"title": fmt.Sprintf("Cursor %d", i), "category": "прогулки",
				"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
				"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
			},
		})
		require.Equal(t, 201, r.Status, r.Raw)
		ids = append(ids, r.Body["id"].(string))
	}

	// Насильно ставим ОДИНАКОВЫЙ starts_at всем событиям в уникальном городе.
	// Секундная точность — тот самый tie-кейс.
	sameTime := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	_, err := env.pgPool.Exec(ctx,
		`UPDATE events SET starts_at = $1 WHERE city = $2`, sameTime, uniqueCity)
	require.NoError(t, err)

	// Читаем ленту с limit=2, пагинируем по next_cursor.
	seen := map[string]bool{}
	pageCount := 0
	var cursor string

	for {
		pageCount++
		require.LessOrEqual(t, pageCount, 10, "слишком много страниц — пагинация зациклилась")

		params := url.Values{}
		params.Set("city", uniqueCity)
		params.Set("limit", "2")
		if cursor != "" {
			params.Set("cursor", cursor)
		}

		r := env.srv.do(t, httpReq{
			method: "GET", path: "/api/v1/events?" + params.Encode(),
		})
		require.Equal(t, 200, r.Status, r.Raw)

		events := r.Body["events"].([]any)
		for _, e := range events {
			id := e.(map[string]any)["id"].(string)
			require.False(t, seen[id], "дубликат события %s в пагинации", id)
			seen[id] = true
		}

		next, _ := r.Body["next_cursor"].(string)
		if next == "" {
			break
		}

		// Защита от бесконечного цикла: сервер вернул тот же курсор,
		// что мы послали, — пагинация не двигается.
		require.NotEqual(t, cursor, next, "курсор не двигается")
		cursor = next
	}

	require.Len(t, seen, 5, "должны увидеть все 5 событий, seen=%v", seen)
	for _, id := range ids {
		require.True(t, seen[id], "событие %s пропало из пагинации", id)
	}
}

// Backward-compat: старый ?cursor_time= (только starts_at) тоже работает.
func TestFeedCursorLegacyCursorTime(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("cl_o"), "clo_"+shortID())
	uniqueCity := "Саратов-" + shortID()

	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Legacy", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)

	// Пустая страница с cursor_time в будущем — должно быть 200 и пусто.
	future := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339)
	resp := env.srv.do(t, httpReq{
		method: "GET",
		path:   "/api/v1/events?city=" + url.QueryEscape(uniqueCity) + "&cursor_time=" + url.QueryEscape(future),
	})
	require.Equal(t, 200, resp.Status, resp.Raw)
	require.Empty(t, resp.Body["events"].([]any))
}
