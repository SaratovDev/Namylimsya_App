package integration

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
)

func TestAdminAuditLog(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	adminEmail := uniqueEmail("al_a")
	userEmail := uniqueEmail("al_u")
	env.registerAndLogin(t, adminEmail, "ala_"+shortID())
	env.registerAndLogin(t, userEmail, "alu_"+shortID())

	adminUser, _ := env.queries.GetUserByEmail(ctx, adminEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: adminUser.ID, Role: "admin",
	})
	adminToken, _ := env.loginRaw(t, adminEmail, "secret123")

	targetUser, _ := env.queries.GetUserByEmail(ctx, userEmail)

	// Действие 1: бан (создаёт запись audit log).
	banResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + targetUser.ID.String() + "/ban",
		token: adminToken,
		body:  map[string]any{"banned": true, "reason": "spam"},
	})
	require.Equal(t, 200, banResp.Status, banResp.Raw)

	// Действие 2: разбан (ещё одна запись).
	unbanResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + targetUser.ID.String() + "/ban",
		token: adminToken,
		body:  map[string]any{"banned": false},
	})
	require.Equal(t, 200, unbanResp.Status, unbanResp.Raw)

	// 1. Все записи.
	all := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/audit-log",
		token: adminToken,
	})
	require.Equal(t, 200, all.Status, all.Raw)
	entries := all.Body["entries"].([]any)
	require.GreaterOrEqual(t, len(entries), 2, "must contain at least ban + unban")

	// Записи содержат поля.
	first := entries[0].(map[string]any)
	require.NotEmpty(t, first["id"])
	require.NotEmpty(t, first["admin_id"])
	require.NotEmpty(t, first["admin_email"])
	require.NotEmpty(t, first["action"])
	require.NotEmpty(t, first["target_type"])
	require.NotEmpty(t, first["target_id"])
	require.NotNil(t, first["created_at"])

	// 2. Фильтр по admin_id.
	byAdmin := env.srv.do(t, httpReq{
		method: "GET",
		path:   "/api/v1/admin/audit-log?admin_id=" + url.QueryEscape(adminUser.ID.String()),
		token:  adminToken,
	})
	require.Equal(t, 200, byAdmin.Status, byAdmin.Raw)
	adminEntries := byAdmin.Body["entries"].([]any)
	require.GreaterOrEqual(t, len(adminEntries), 2)
	for _, e := range adminEntries {
		m := e.(map[string]any)
		require.Equal(t, adminUser.ID.String(), m["admin_id"])
	}

	// 3. Фильтр по action.
	byAction := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/audit-log?action=ban_user",
		token: adminToken,
	})
	require.Equal(t, 200, byAction.Status, byAction.Raw)
	banEntries := byAction.Body["entries"].([]any)
	require.GreaterOrEqual(t, len(banEntries), 1)
	for _, e := range banEntries {
		m := e.(map[string]any)
		require.Equal(t, "ban_user", m["action"])
	}

	// 4. Фильтр по target_type.
	byTarget := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/audit-log?target_type=user",
		token: adminToken,
	})
	require.Equal(t, 200, byTarget.Status, byTarget.Raw)
	require.GreaterOrEqual(t, len(byTarget.Body["entries"].([]any)), 2)

	// 5. Фильтр по несуществующему admin_id — пусто.
	empty := env.srv.do(t, httpReq{
		method: "GET",
		path:   "/api/v1/admin/audit-log?admin_id=00000000-0000-0000-0000-000000000042",
		token:  adminToken,
	})
	require.Equal(t, 200, empty.Status)
	require.Empty(t, empty.Body["entries"].([]any))
}

func TestAdminAuditLogForbiddenForModerator(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	modEmail := uniqueEmail("al_mod")
	env.registerAndLogin(t, modEmail, "alm_"+shortID())
	modUser, _ := env.queries.GetUserByEmail(ctx, modEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: modUser.ID, Role: "moderator",
	})
	modToken, _ := env.loginRaw(t, modEmail, "secret123")

	// Модератор — 403 (только admin).
	resp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/audit-log",
		token: modToken,
	})
	require.Equal(t, 403, resp.Status, resp.Raw)

	// Гость — 401.
	guest := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/audit-log",
	})
	require.Equal(t, 401, guest.Status, guest.Raw)
}
