package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Create отклоняет невалидные координаты.
func TestCreateEventRejectsInvalidCoords(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("coord_o"), "coordo_"+shortID())

	cases := []struct {
		name    string
		lat     float64
		lng     float64
		wantMsg string
	}{
		{"lat>90", 91.0, 46.03, "Широта"},
		{"lat<-90", -91.0, 46.03, "Широта"},
		{"lng>180", 51.53, 181.0, "Долгота"},
		{"lng<-180", 51.53, -181.0, "Долгота"},
		{"0,0", 0.0, 0.0, "Укажите координаты"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := env.srv.do(t, httpReq{
				method: "POST", path: "/api/v1/events", token: token,
				body: map[string]any{
					"title": "Bad coords", "category": "прогулки",
					"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
					"city":      "Саратов", "lat": c.lat, "lng": c.lng,
				},
			})
			require.Equal(t, http.StatusBadRequest, r.Status, r.Raw)
			require.Contains(t, r.Body["message"], c.wantMsg)
		})
	}
}

// Update отклоняет невалидные координаты.
func TestUpdateEventRejectsInvalidCoords(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("uc_o"), "uco_"+shortID())

	// Создаём валидное событие.
	create := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: token,
		body: map[string]any{
			"title": "Valid", "category": "прогулки",
			"starts_at": futureTime(48 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, create.Status, create.Raw)
	eventID := create.Body["id"].(string)

	// Пытаемся обновить на невалидные.
	r := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/events/" + eventID, token: token,
		body: map[string]any{
			"title": "Valid", "category": "прогулки",
			"starts_at": futureTime(48 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 999.0, "lng": 46.03,
		},
	})
	require.Equal(t, http.StatusBadRequest, r.Status, r.Raw)
	require.Contains(t, r.Body["message"], "Широта")
}

// Валидные координаты — 201.
func TestCreateEventAcceptsValidCoords(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("vc_o"), "vco_"+shortID())

	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: token,
		body: map[string]any{
			"title": "Valid", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)
}
