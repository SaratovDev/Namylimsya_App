package integration

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCategoryNormalizedOnCreate(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("cat_norm"), "cn_"+shortID())

	// Создаём с «Прогулки» (заглавная) — сервис нормализует в «прогулки».
	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: token,
		body: map[string]any{
			"title": "Нормализация", "category": "Прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	require.Equal(t, "прогулки", resp.Body["category"])
	eventID := resp.Body["id"].(string)

	// Проверяем через GET: категория — slug.
	getResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: token,
	})
	require.Equal(t, 200, getResp.Status, getResp.Raw)
	require.Equal(t, "прогулки", getResp.Body["category"])
}

func TestInvalidCategoryRejected(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("cat_inv"), "ci_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: token,
		body: map[string]any{
			"title": "Плохая категория", "category": "несуществующая",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 400, resp.Status, resp.Raw)
	require.Contains(t, resp.Body["message"], "категория")
}

func TestFeedCategoryFilterNormalized(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("cat_org"), "co_"+shortID())

	uniqueCity := "Саратов-" + shortID()

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Категорийное", "category": "Прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	found := func(rawQuery string) bool {
		r := env.srv.do(t, httpReq{
			method: "GET",
			path:   "/api/v1/events?city=" + url.QueryEscape(uniqueCity) + "&category=" + url.QueryEscape(rawQuery),
		})
		require.Equal(t, 200, r.Status, r.Raw)
		for _, e := range r.Body["events"].([]any) {
			if e.(map[string]any)["id"] == eventID {
				return true
			}
		}
		return false
	}

	require.True(t, found("прогулки"), "lowercase slug должен находить")
	require.True(t, found("Прогулки"), "с заглавной должен находить после нормализации")

	r := env.srv.do(t, httpReq{
		method: "GET",
		path:   "/api/v1/events?city=" + url.QueryEscape(uniqueCity) + "&category=xyz",
	})
	require.Equal(t, 200, r.Status)
	events := r.Body["events"].([]any)
	require.Empty(t, events)
}
