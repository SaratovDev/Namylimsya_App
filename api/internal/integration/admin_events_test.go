package integration

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
)

// setupAdmin регистрирует admin'а и возвращает его токен и id.
func setupAdmin(t *testing.T, env *testEnv) (string, string) {
	t.Helper()
	ctx := context.Background()
	email := uniqueEmail("ae_admin")
	env.registerAndLogin(t, email, "aea_"+shortID())
	u, err := env.queries.GetUserByEmail(ctx, email)
	require.NoError(t, err)
	_, err = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: u.ID, Role: "admin",
	})
	require.NoError(t, err)
	token, _ := env.loginRaw(t, email, "secret123")
	return token, u.ID.String()
}

func TestAdminGetEventDetails(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _ := setupAdmin(t, env)
	orgToken := env.registerAndLogin(t, uniqueEmail("ae_org"), "aeo_"+shortID())
	appToken := env.registerAndLogin(t, uniqueEmail("ae_app"), "aea2_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "AE", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// Pending заявка.
	env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: appToken,
	})

	detail := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/events/" + eventID, token: adminToken,
	})
	require.Equal(t, 200, detail.Status, detail.Raw)

	event := detail.Body["event"].(map[string]any)
	require.Equal(t, eventID, event["id"])
	require.Equal(t, "AE", event["title"])
	require.Equal(t, "active", event["status"])
	require.EqualValues(t, 0, event["participants_count"])
	require.EqualValues(t, 1, event["pending_count"])
	require.NotNil(t, event["organizer_email"])

	apps := detail.Body["applications"].([]any)
	require.Len(t, apps, 1)

	// Гость — 401.
	guest := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/events/" + eventID,
	})
	require.Equal(t, 401, guest.Status, guest.Raw)

	// Обычный user — 403.
	plainToken := env.registerAndLogin(t, uniqueEmail("ae_plain"), "aep_"+shortID())
	plain := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/events/" + eventID,
		token: plainToken,
	})
	require.Equal(t, 403, plain.Status, plain.Raw)

	// Несуществующее — 404.
	nf := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/events/00000000-0000-0000-0000-000000000099",
		token: adminToken,
	})
	require.Equal(t, 404, nf.Status)
}

func TestAdminCancelEvent(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, adminID := setupAdmin(t, env)
	orgToken := env.registerAndLogin(t, uniqueEmail("cae_org"), "cae_"+shortID())
	appToken := env.registerAndLogin(t, uniqueEmail("cae_app"), "cae2_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "CAE", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// Applicant подаёт и получает approve.
	applyResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: appToken,
	})
	require.Equal(t, 201, applyResp.Status)
	appID := applyResp.Body["id"].(string)
	env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID + "/approve", token: orgToken,
	})

	// Админ отменяет.
	cancelResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/events/" + eventID + "/cancel",
		token: adminToken,
		body:  map[string]any{"reason": "нарушение правил"},
	})
	require.Equal(t, 200, cancelResp.Status, cancelResp.Raw)
	require.Equal(t, "cancelled", cancelResp.Body["status"])
	require.Equal(t, "нарушение правил", cancelResp.Body["cancel_reason"])

	// Заявка каскадно отменена.
	var status string
	err := env.pgPool.QueryRow(context.Background(),
		"SELECT status FROM applications WHERE id = $1", appID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "cancelled", status)

	// Audit log записан.
	var count int
	err = env.pgPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM admin_audit_log 
		 WHERE admin_id = $1 AND action = 'cancel_event' AND target_id = $2`,
		adminID, eventID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Повторная отмена — 400.
	again := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/events/" + eventID + "/cancel",
		token: adminToken,
		body:  map[string]any{"reason": "x"},
	})
	require.Equal(t, 400, again.Status)
}

func TestAdminHideEvent(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _ := setupAdmin(t, env)
	orgToken := env.registerAndLogin(t, uniqueEmail("he_org"), "he_"+shortID())

	uniqueCity := "Саратов-" + shortID()

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "HE", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// В ленте есть.
	feed := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events?city=" + url.QueryEscape(uniqueCity),
	})
	require.Equal(t, 200, feed.Status)
	require.Len(t, feed.Body["events"].([]any), 1)

	// Скрываем.
	hide := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/events/" + eventID + "/hide",
		token: adminToken,
	})
	require.Equal(t, 200, hide.Status, hide.Raw)

	// В ленте нет.
	feed2 := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events?city=" + url.QueryEscape(uniqueCity),
	})
	require.Equal(t, 200, feed2.Status)
	require.Len(t, feed2.Body["events"].([]any), 0)

	// По прямой ссылке — недоступно гостям (hidden=true).
	direct := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID,
	})
	require.Equal(t, 404, direct.Status, "скрытое событие не должно открываться по прямой ссылке")

	// Организатор видит своё скрытое событие.
	directOrg := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: orgToken,
	})
	require.Equal(t, 200, directOrg.Status, "организатор должен видеть своё скрытое событие")

	// Unhide — снова в ленте.
	unhide := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/events/" + eventID + "/unhide",
		token: adminToken,
	})
	require.Equal(t, 200, unhide.Status)

	feed3 := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events?city=" + url.QueryEscape(uniqueCity),
	})
	require.Equal(t, 200, feed3.Status)
	require.Len(t, feed3.Body["events"].([]any), 1)
}

func TestModeratorCanCancelEvent(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	// Модератор.
	modEmail := uniqueEmail("mce_mod")
	env.registerAndLogin(t, modEmail, "mce_"+shortID())
	modUser, _ := env.queries.GetUserByEmail(ctx, modEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: modUser.ID, Role: "moderator",
	})
	modToken, _ := env.loginRaw(t, modEmail, "secret123")

	orgToken := env.registerAndLogin(t, uniqueEmail("mce_org"), "mceo_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "MCE", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status)
	eventID := resp.Body["id"].(string)

	cancelResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/events/" + eventID + "/cancel",
		token: modToken,
		body:  map[string]any{"reason": "test by moderator"},
	})
	require.Equal(t, 200, cancelResp.Status, cancelResp.Raw)
	require.Equal(t, "cancelled", cancelResp.Body["status"])
}
