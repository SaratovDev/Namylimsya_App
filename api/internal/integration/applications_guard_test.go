package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Повторный Reject на уже approved — 400, а не 200.
func TestRejectApprovedApplicationFails(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("rg_o"), "rgo_"+shortID())
	appToken := env.registerAndLogin(t, uniqueEmail("rg_a"), "rga_"+shortID())

	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Reject guard", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)
	eventID := r.Body["id"].(string)

	apply := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: appToken,
	})
	require.Equal(t, 201, apply.Status, apply.Raw)
	appID := apply.Body["id"].(string)

	approve := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID + "/approve", token: orgToken,
	})
	require.Equal(t, 200, approve.Status, approve.Raw)

	reject := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID + "/reject", token: orgToken,
	})
	require.Equal(t, 400, reject.Status, reject.Raw)
}

// Повторный Approve на уже cancelled — 400.
func TestApproveCancelledApplicationFails(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("ag_o"), "ago_"+shortID())
	appToken := env.registerAndLogin(t, uniqueEmail("ag_a"), "aga_"+shortID())

	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Approve guard", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)
	eventID := r.Body["id"].(string)

	apply := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: appToken,
	})
	require.Equal(t, 201, apply.Status, apply.Raw)
	appID := apply.Body["id"].(string)

	// Applicant отменяет.
	cancel := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/applications/" + appID + "/cancel",
		token: appToken, body: map[string]any{"reason": "test"},
	})
	require.Equal(t, 200, cancel.Status, cancel.Raw)

	// Organizer пытается одобрить — не pending.
	approve := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID + "/approve", token: orgToken,
	})
	require.Equal(t, 400, approve.Status, approve.Raw)
}

// Повторный Cancel — 400.
func TestCancelCancelledApplicationFails(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("cg_o"), "cgo_"+shortID())
	appToken := env.registerAndLogin(t, uniqueEmail("cg_a"), "cga_"+shortID())

	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Cancel guard", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)
	eventID := r.Body["id"].(string)

	apply := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: appToken,
	})
	require.Equal(t, 201, apply.Status, apply.Raw)
	appID := apply.Body["id"].(string)

	first := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/applications/" + appID + "/cancel",
		token: appToken, body: map[string]any{"reason": "first"},
	})
	require.Equal(t, 200, first.Status, first.Raw)

	second := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/applications/" + appID + "/cancel",
		token: appToken, body: map[string]any{"reason": "second"},
	})
	require.Equal(t, 400, second.Status, second.Raw)
}
