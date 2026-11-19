package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
)

// setupAdminComplaints создаёт admin'а и возвращает (token, id).
func setupAdminComplaints(t *testing.T, env *testEnv) (string, string) {
	t.Helper()
	ctx := context.Background()
	email := uniqueEmail("ac_admin")
	env.registerAndLogin(t, email, "aca_"+shortID())
	u, err := env.queries.GetUserByEmail(ctx, email)
	require.NoError(t, err)
	_, err = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: u.ID, Role: "admin",
	})
	require.NoError(t, err)
	token, _ := env.loginRaw(t, email, "secret123")
	return token, u.ID.String()
}

func createEventAndComplaintOnEvent(t *testing.T, env *testEnv) (adminToken, eventID, complaintID string) {
	t.Helper()
	adminToken, _ = setupAdminComplaints(t, env)
	orgToken := env.registerAndLogin(t, uniqueEmail("ace_org"), "aceo_"+shortID())
	reporterToken := env.registerAndLogin(t, uniqueEmail("ace_rep"), "acer_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Complaint target", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID = resp.Body["id"].(string)

	// Жалоба на событие.
	compResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/complaints", token: reporterToken,
		body: map[string]any{
			"target_type": "event",
			"target_id":   eventID,
			"reason":      "спам",
		},
	})
	require.Equal(t, 201, compResp.Status, compResp.Raw)
	complaintID = compResp.Body["id"].(string)

	return
}

func TestAdminGetComplaintEvent(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, eventID, complaintID := createEventAndComplaintOnEvent(t, env)

	resp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/complaints/" + complaintID,
		token: adminToken,
	})
	require.Equal(t, 200, resp.Status, resp.Raw)

	c := resp.Body["complaint"].(map[string]any)
	require.Equal(t, "event", c["target_type"])
	require.Equal(t, "open", c["status"])
	require.Equal(t, "спам", c["reason"])

	ev := resp.Body["event"].(map[string]any)
	require.Equal(t, eventID, ev["id"])
	require.Equal(t, "Complaint target", ev["title"])
	require.NotEmpty(t, ev["organizer_name"])

	// Гость без токена — 401 (AuthRequired).
	g := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/complaints/" + complaintID,
	})
	require.Equal(t, 401, g.Status, g.Raw)

	// Обычный user (без роли admin/mod) — 403 (RequireRole).
	plainToken := env.registerAndLogin(t, uniqueEmail("ac_plain"), "acp_"+shortID())
	plain := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/complaints/" + complaintID,
		token: plainToken,
	})
	require.Equal(t, 403, plain.Status, plain.Raw)

	// Несуществующая — 404.
	nf := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/complaints/00000000-0000-0000-0000-000000000088",
		token: adminToken,
	})
	require.Equal(t, 404, nf.Status)
}

func TestAdminResolveComplaint(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _, complaintID := createEventAndComplaintOnEvent(t, env)

	// Resolve с комментарием.
	resp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/complaints/" + complaintID + "/resolve",
		token: adminToken,
		body:  map[string]any{"comment": "нарушение подтверждено"},
	})
	require.Equal(t, 200, resp.Status, resp.Raw)
	require.Equal(t, "resolved", resp.Body["status"])
	require.Equal(t, "нарушение подтверждено", resp.Body["resolution_comment"])
	require.NotNil(t, resp.Body["resolved_at"])
	require.NotNil(t, resp.Body["resolved_by"])

	// Audit log записан.
	var count int
	err := env.pgPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM admin_audit_log 
		 WHERE action = 'resolve_complaint' AND target_id = $1`, complaintID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Повторный resolve — 400.
	again := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/complaints/" + complaintID + "/resolve",
		token: adminToken,
		body:  map[string]any{"comment": "x"},
	})
	require.Equal(t, 400, again.Status)
}

func TestAdminRejectComplaint(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _, complaintID := createEventAndComplaintOnEvent(t, env)

	resp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/complaints/" + complaintID + "/reject",
		token: adminToken,
		body:  map[string]any{"comment": "необоснованно"},
	})
	require.Equal(t, 200, resp.Status, resp.Raw)
	require.Equal(t, "rejected", resp.Body["status"])
	require.Equal(t, "необоснованно", resp.Body["resolution_comment"])

	// После reject resolve невозможен.
	res := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/complaints/" + complaintID + "/resolve",
		token: adminToken,
	})
	require.Equal(t, 400, res.Status)
}

func TestAdminDeleteMessage(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _ := setupAdminComplaints(t, env)
	orgToken := env.registerAndLogin(t, uniqueEmail("dm_org"), "dmo_"+shortID())

	// Создаём событие + сообщение.
	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "DM", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status)
	eventID := resp.Body["id"].(string)

	msgResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/messages", token: orgToken,
		body: map[string]any{"text": "плохое сообщение"},
	})
	require.Equal(t, 201, msgResp.Status)
	msgID := msgResp.Body["id"].(string)

	// Сообщение в истории.
	hist := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: orgToken,
	})
	require.Equal(t, 200, hist.Status)
	require.Len(t, hist.Body["messages"].([]any), 1)

	// Удаляем.
	del := env.srv.do(t, httpReq{
		method: "DELETE", path: "/api/v1/admin/messages/" + msgID,
		token: adminToken,
	})
	require.Equal(t, 204, del.Status, del.Raw)

	// В истории нет.
	hist2 := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: orgToken,
	})
	require.Equal(t, 200, hist2.Status)
	require.Len(t, hist2.Body["messages"].([]any), 0)

	// В БД deleted_at проставлен.
	var deletedAt *time.Time
	err := env.pgPool.QueryRow(context.Background(),
		`SELECT deleted_at FROM chat_messages WHERE id = $1`, msgID).Scan(&deletedAt)
	require.NoError(t, err)
	require.NotNil(t, deletedAt)

	// Audit log записан.
	var count int
	err = env.pgPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM admin_audit_log 
		 WHERE action = 'delete_message' AND target_id = $1`, msgID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Повторное удаление — 404 (уже удалено).
	del2 := env.srv.do(t, httpReq{
		method: "DELETE", path: "/api/v1/admin/messages/" + msgID,
		token: adminToken,
	})
	require.Equal(t, 404, del2.Status)
}

func TestModeratorCanResolveComplaint(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	// Модератор.
	modEmail := uniqueEmail("mcr_mod")
	env.registerAndLogin(t, modEmail, "mcr_"+shortID())
	modUser, _ := env.queries.GetUserByEmail(ctx, modEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: modUser.ID, Role: "moderator",
	})
	modToken, _ := env.loginRaw(t, modEmail, "secret123")

	orgToken := env.registerAndLogin(t, uniqueEmail("mcr_org"), "mcro_"+shortID())
	reporterToken := env.registerAndLogin(t, uniqueEmail("mcr_rep"), "mcrr_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "MCR", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status)
	eventID := resp.Body["id"].(string)

	compResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/complaints", token: reporterToken,
		body: map[string]any{
			"target_type": "event",
			"target_id":   eventID,
			"reason":      "spam",
		},
	})
	require.Equal(t, 201, compResp.Status)
	complaintID := compResp.Body["id"].(string)

	resolve := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/complaints/" + complaintID + "/resolve",
		token: modToken,
		body:  map[string]any{"comment": "by moderator"},
	})
	require.Equal(t, 200, resolve.Status, resolve.Raw)
	require.Equal(t, "resolved", resolve.Body["status"])
}
