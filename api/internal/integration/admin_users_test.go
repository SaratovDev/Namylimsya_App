package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
)

func TestAdminGetUserDetails(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	adminEmail := uniqueEmail("admu")
	userEmail := uniqueEmail("admu_target")

	adminToken := env.registerAndLogin(t, adminEmail, "admu_"+shortID())
	userToken := env.registerAndLogin(t, userEmail, "admu_t_"+shortID())

	// Назначаем admin'а через queries.
	adminUser, err := env.queries.GetUserByEmail(ctx, adminEmail)
	require.NoError(t, err)
	_, err = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: adminUser.ID, Role: "admin",
	})
	require.NoError(t, err)
	adminToken, _ = env.loginRaw(t, adminEmail, "secret123")

	targetUser, err := env.queries.GetUserByEmail(ctx, userEmail)
	require.NoError(t, err)

	// 1. Админ получает детали.
	resp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/users/" + targetUser.ID.String(),
		token: adminToken,
	})
	require.Equal(t, 200, resp.Status, resp.Raw)

	user := resp.Body["user"].(map[string]any)
	require.Equal(t, targetUser.ID.String(), user["id"])
	require.Equal(t, userEmail, user["email"])
	require.NotContains(t, user, "password_hash", "password_hash must not leak")

	stats := resp.Body["stats"].(map[string]any)
	require.EqualValues(t, 0, stats["organized_events"])
	require.EqualValues(t, 0, stats["applications_total"])

	// 2. Обычный user — 403.
	meResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/users/" + targetUser.ID.String(),
		token: userToken,
	})
	require.Equal(t, 403, meResp.Status)

	// 3. Несуществующий — 404.
	notFoundResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/admin/users/00000000-0000-0000-0000-000000000001",
		token: adminToken,
	})
	require.Equal(t, 404, notFoundResp.Status)
}

func TestAdminBanWithExpiresAt(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	adminEmail := uniqueEmail("bexp_a")
	userEmail := uniqueEmail("bexp_u")

	adminToken := env.registerAndLogin(t, adminEmail, "bexpa_"+shortID())
	env.registerAndLogin(t, userEmail, "bexpu_"+shortID())

	adminUser, _ := env.queries.GetUserByEmail(ctx, adminEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: adminUser.ID, Role: "admin",
	})
	adminToken, _ = env.loginRaw(t, adminEmail, "secret123")

	targetUser, _ := env.queries.GetUserByEmail(ctx, userEmail)

	// Бан на 1 час.
	exp := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	resp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + targetUser.ID.String() + "/ban",
		token: adminToken,
		body:  map[string]any{"banned": true, "reason": "spam", "expires_at": exp},
	})
	require.Equal(t, 200, resp.Status, resp.Raw)
	require.Equal(t, true, resp.Body["is_banned"])
	require.NotNil(t, resp.Body["ban_expires_at"])

	// В БД ban_expires_at заполнен.
	var banExpires *time.Time
	err := env.pgPool.QueryRow(ctx,
		`SELECT ban_expires_at FROM users WHERE id = $1`, targetUser.ID).
		Scan(&banExpires)
	require.NoError(t, err)
	require.NotNil(t, banExpires)

	// Без reason → 400.
	noReason := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + targetUser.ID.String() + "/ban",
		token: adminToken,
		body:  map[string]any{"banned": true, "reason": "   "},
	})
	require.Equal(t, 400, noReason.Status)

	// Прошлое expires_at → 400.
	pastExp := time.Now().Add(-time.Hour).Format(time.RFC3339)
	pastResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + targetUser.ID.String() + "/ban",
		token: adminToken,
		body:  map[string]any{"banned": true, "reason": "x", "expires_at": pastExp},
	})
	require.Equal(t, 400, pastResp.Status)
}

func TestAdminBanSelfForbidden(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	adminEmail := uniqueEmail("self_b")

	env.registerAndLogin(t, adminEmail, "selfb_"+shortID())
	adminUser, _ := env.queries.GetUserByEmail(ctx, adminEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: adminUser.ID, Role: "admin",
	})
	adminToken, _ := env.loginRaw(t, adminEmail, "secret123")

	resp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + adminUser.ID.String() + "/ban",
		token: adminToken,
		body:  map[string]any{"banned": true, "reason": "test"},
	})
	require.Equal(t, 400, resp.Status, resp.Raw)
	require.Contains(t, resp.Body["message"], "самого себя")
}

func TestAdminBanWritesAuditLog(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	adminEmail := uniqueEmail("audit_a")
	userEmail := uniqueEmail("audit_u")

	env.registerAndLogin(t, adminEmail, "aa_"+shortID())
	env.registerAndLogin(t, userEmail, "au_"+shortID())

	adminUser, _ := env.queries.GetUserByEmail(ctx, adminEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: adminUser.ID, Role: "admin",
	})
	adminToken, _ := env.loginRaw(t, adminEmail, "secret123")

	targetUser, _ := env.queries.GetUserByEmail(ctx, userEmail)

	resp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + targetUser.ID.String() + "/ban",
		token: adminToken,
		body:  map[string]any{"banned": true, "reason": "test audit"},
	})
	require.Equal(t, 200, resp.Status, resp.Raw)

	// В audit log есть запись.
	var count int
	err := env.pgPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM admin_audit_log 
		 WHERE admin_id = $1 AND action = 'ban_user' AND target_id = $2`,
		adminUser.ID, targetUser.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "audit log entry for ban must exist")
}

func TestModeratorCannotBanAdmin(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	modEmail := uniqueEmail("mod_m")
	adminEmail := uniqueEmail("mod_a")

	env.registerAndLogin(t, modEmail, "mm_"+shortID())
	env.registerAndLogin(t, adminEmail, "ma_"+shortID())

	modUser, _ := env.queries.GetUserByEmail(ctx, modEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: modUser.ID, Role: "moderator",
	})
	modToken, _ := env.loginRaw(t, modEmail, "secret123")

	adminUser, _ := env.queries.GetUserByEmail(ctx, adminEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: adminUser.ID, Role: "admin",
	})

	resp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + adminUser.ID.String() + "/ban",
		token: modToken,
		body:  map[string]any{"banned": true, "reason": "test"},
	})
	require.Equal(t, 400, resp.Status, resp.Raw)
	require.Contains(t, resp.Body["message"], "администратора")
}
func TestModeratorCanBanRegularUser(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	modEmail := uniqueEmail("modb_m")
	userEmail := uniqueEmail("modb_u")

	env.registerAndLogin(t, modEmail, "modb_"+shortID())
	env.registerAndLogin(t, userEmail, "mbu_"+shortID())

	modUser, _ := env.queries.GetUserByEmail(ctx, modEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: modUser.ID, Role: "moderator",
	})
	modToken, _ := env.loginRaw(t, modEmail, "secret123")

	targetUser, _ := env.queries.GetUserByEmail(ctx, userEmail)

	resp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + targetUser.ID.String() + "/ban",
		token: modToken,
		body:  map[string]any{"banned": true, "reason": "spam by moderator"},
	})
	require.Equal(t, 200, resp.Status, resp.Raw)
	require.Equal(t, true, resp.Body["is_banned"])

	// Разбан модератором — тоже разрешён.
	unbanResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + targetUser.ID.String() + "/ban",
		token: modToken,
		body:  map[string]any{"banned": false},
	})
	require.Equal(t, 200, unbanResp.Status, unbanResp.Raw)
	require.Equal(t, false, unbanResp.Body["is_banned"])
}
