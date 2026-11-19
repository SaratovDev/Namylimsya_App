package integration

import (
	"context"
	"testing"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/stretchr/testify/require"
)

// createTicket создаёт тикет через публичный API и возвращает его id.
func createTicket(t *testing.T, env *testEnv, token, typ, subject, msg string) string {
	t.Helper()
	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/support/tickets", token: token,
		body: map[string]any{"type": typ, "subject": subject, "message": msg},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	return resp.Body["id"].(string)
}

func TestAdminGetTicket(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _, _ := createEventAndComplaintOnEvent(t, env)
	userToken := env.registerAndLogin(t, uniqueEmail("tkt_u"), "tk_u_"+shortID())

	ticketID := createTicket(t, env, userToken, "suggestion", "Идея", "Добавьте тёмную тему")

	d := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/support/tickets/" + ticketID,
		token: adminToken,
	})
	require.Equal(t, 200, d.Status, d.Raw)

	tk := d.Body["ticket"].(map[string]any)
	require.Equal(t, ticketID, tk["id"])
	require.Equal(t, "suggestion", tk["type"])
	require.Equal(t, "Идея", tk["subject"])
	require.Equal(t, "open", tk["status"])
	require.NotNil(t, tk["user_email"])

	// Replies пустой (но присутствует как массив).
	replies := d.Body["replies"].([]any)
	require.Empty(t, replies)

	// Гость — 401.
	g := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/support/tickets/" + ticketID,
	})
	require.Equal(t, 401, g.Status, g.Raw)

	// Несуществующий — 404.
	nf := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/support/tickets/00000000-0000-0000-0000-000000000077",
		token: adminToken,
	})
	require.Equal(t, 404, nf.Status)
}

func TestAdminResolveTicket(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _, _ := createEventAndComplaintOnEvent(t, env)
	userToken := env.registerAndLogin(t, uniqueEmail("rt_u"), "rtu_"+shortID())

	ticketID := createTicket(t, env, userToken, "suggestion", "Subject", "Body")

	resp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/support/tickets/" + ticketID + "/resolve",
		token: adminToken,
		body:  map[string]any{"comment": "принято в работу"},
	})
	require.Equal(t, 200, resp.Status, resp.Raw)
	require.Equal(t, "resolved", resp.Body["status"])
	require.Equal(t, "принято в работу", resp.Body["resolution_comment"])
	require.NotNil(t, resp.Body["resolved_at"])
	// Audit log.
	var count int
	err := env.pgPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM admin_audit_log WHERE action = 'resolve_ticket' AND target_id = $1`,
		ticketID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Повторно — 400.
	again := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/support/tickets/" + ticketID + "/resolve",
		token: adminToken,
		body:  map[string]any{"comment": "x"},
	})
	require.Equal(t, 400, again.Status)
}

func TestAdminRejectTicket(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _, _ := createEventAndComplaintOnEvent(t, env)
	userToken := env.registerAndLogin(t, uniqueEmail("rjt_u"), "rjtu_"+shortID())

	ticketID := createTicket(t, env, userToken, "appeal", "Апелляция", "Разбаньте")

	resp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/support/tickets/" + ticketID + "/reject",
		token: adminToken,
		body:  map[string]any{"comment": "необоснованно"},
	})
	require.Equal(t, 200, resp.Status, resp.Raw)
	require.Equal(t, "rejected", resp.Body["status"])
	require.Equal(t, "необоснованно", resp.Body["resolution_comment"])

	// resolve после reject — 400.
	res := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/support/tickets/" + ticketID + "/resolve",
		token: adminToken,
	})
	require.Equal(t, 400, res.Status)
}

func TestAdminReplyToTicket(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _, _ := createEventAndComplaintOnEvent(t, env)
	userToken := env.registerAndLogin(t, uniqueEmail("rp_u"), "rpu_"+shortID())

	ticketID := createTicket(t, env, userToken, "suggestion", "Тема", "Вопрос")

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/support/tickets/" + ticketID + "/reply",
		token: adminToken,
		body:  map[string]any{"body": "Ответ от админа"},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	require.Equal(t, "Ответ от админа", resp.Body["body"])
	require.NotNil(t, resp.Body["admin_id"])
	require.NotEmpty(t, resp.Body["admin_email"])

	// Reply сохранён в БД.
	var body string
	err := env.pgPool.QueryRow(context.Background(),
		`SELECT body FROM ticket_replies WHERE ticket_id = $1`, ticketID).Scan(&body)
	require.NoError(t, err)
	require.Equal(t, "Ответ от админа", body)

	// В деталях тикета — 1 reply.
	d := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/support/tickets/" + ticketID,
		token: adminToken,
	})
	require.Equal(t, 200, d.Status)
	replies := d.Body["replies"].([]any)
	require.Len(t, replies, 1)
	require.Equal(t, "Ответ от админа", replies[0].(map[string]any)["body"])

	// Пустой body — 400.
	empty := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/support/tickets/" + ticketID + "/reply",
		token: adminToken,
		body:  map[string]any{"body": "   "},
	})
	require.Equal(t, 400, empty.Status)

	// Audit log.
	var count int
	err = env.pgPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM admin_audit_log WHERE action = 'reply_ticket' AND target_id = $1`,
		ticketID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

// Жалоба автоматически создаёт тикет с complaint_id.
func TestComplaintCreatesTicket(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	orgToken := env.registerAndLogin(t, uniqueEmail("cct_org"), "ccto_"+shortID())
	reporterToken := env.registerAndLogin(t, uniqueEmail("cct_rep"), "cctr_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "CCT", "category": "прогулки",
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
			"reason":      "спам",
		},
	})
	require.Equal(t, 201, compResp.Status)
	complaintID := compResp.Body["id"].(string)

	// В БД есть тикет с complaint_id.
	var ticketID string
	var ticketType, ticketStatus string
	err := env.pgPool.QueryRow(ctx,
		`SELECT id, type, status FROM support_tickets WHERE complaint_id = $1`, complaintID).
		Scan(&ticketID, &ticketType, &ticketStatus)
	require.NoError(t, err)
	require.NotEmpty(t, ticketID)
	require.Equal(t, "complaint", ticketType)
	require.Equal(t, "open", ticketStatus)
}

func TestModeratorCanReplyToTicket(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	modEmail := uniqueEmail("mrt_mod")
	env.registerAndLogin(t, modEmail, "mrt_"+shortID())
	modUser, _ := env.queries.GetUserByEmail(ctx, modEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: modUser.ID, Role: "moderator",
	})
	modToken, _ := env.loginRaw(t, modEmail, "secret123")

	userToken := env.registerAndLogin(t, uniqueEmail("mrt_u"), "mrtu_"+shortID())
	ticketID := createTicket(t, env, userToken, "suggestion", "Тема", "Текст")

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/support/tickets/" + ticketID + "/reply",
		token: modToken,
		body:  map[string]any{"body": "Ответ модератора"},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	require.Equal(t, "Ответ модератора", resp.Body["body"])
}
