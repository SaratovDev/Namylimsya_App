package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
)

// Истёкший временный бан не должен блокировать логин.
func TestLoginAllowedAfterBanExpires(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	email := uniqueEmail("bexp")
	env.registerAndLogin(t, email, "bexp_"+shortID())

	user, err := env.queries.GetUserByEmail(ctx, email)
	require.NoError(t, err)

	// Ставим бан с истёкшим сроком напрямую в БД.
	_, err = env.pgPool.Exec(ctx,
		`UPDATE users SET is_banned=true, ban_reason='test',
		 ban_expires_at = now() - interval '1 second' WHERE id = $1`, user.ID)
	require.NoError(t, err)

	// Логин должен пройти: ban_expires_at в прошлом.
	token, _ := env.loginRaw(t, email, "secret123")
	require.NotEmpty(t, token)
}

// Активный временный бан блокирует логин.
func TestLoginBlockedWhileBanActive(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	email := uniqueEmail("bact")
	env.registerAndLogin(t, email, "bact_"+shortID())

	user, err := env.queries.GetUserByEmail(ctx, email)
	require.NoError(t, err)

	// Бан на 1 час в будущем.
	_, err = env.pgPool.Exec(ctx,
		`UPDATE users SET is_banned=true, ban_reason='test',
		 ban_expires_at = now() + interval '1 hour' WHERE id = $1`, user.ID)
	require.NoError(t, err)

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/login",
		body: map[string]any{"email": email, "password": "secret123"},
	})
	require.Equal(t, 403, resp.Status, resp.Raw)
	require.Equal(t, "banned", resp.Body["error"])
}

// Админский бан на 1 час: Login блокируется, Refresh блокируется,
// Redis-флаг не снимается мгновенно.
func TestAdminBanWithExpiryAndRefreshBlocked(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	adminEmail := uniqueEmail("bae_a")
	userEmail := uniqueEmail("bae_u")

	env.registerAndLogin(t, adminEmail, "baea_"+shortID())
	env.registerAndLogin(t, userEmail, "baeu_"+shortID())

	adminUser, _ := env.queries.GetUserByEmail(ctx, adminEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: adminUser.ID, Role: "admin",
	})
	adminToken, _ := env.loginRaw(t, adminEmail, "secret123")

	targetUser, _ := env.queries.GetUserByEmail(ctx, userEmail)

	// Логин target до бана — сохраняем refresh-cookie.
	_, cookiesBefore := env.loginRaw(t, userEmail, "secret123")
	rcBefore := cookieKV(findCookie(cookiesBefore, "refresh_token"))
	require.NotEmpty(t, rcBefore)

	// Бан на 1 час.
	exp := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	banResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + targetUser.ID.String() + "/ban",
		token: adminToken,
		body:  map[string]any{"banned": true, "reason": "test", "expires_at": exp},
	})
	require.Equal(t, 200, banResp.Status, banResp.Raw)

	// Login — 403.
	loginResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/login",
		body: map[string]any{"email": userEmail, "password": "secret123"},
	})
	require.Equal(t, 403, loginResp.Status, loginResp.Raw)
	require.Equal(t, "banned", loginResp.Body["error"])

	// Refresh с прежней cookie — 401.
	refreshResp := env.srv.doRaw(t, httpReq{
		method: "POST", path: "/api/v1/auth/refresh",
		cookies: rcBefore,
	})
	require.Equal(t, 401, refreshResp.StatusCode, "refresh must be revoked after ban")
	defer refreshResp.Body.Close()
}

// Фоновый воркер снимает is_banned для истёкших.
func TestUnbanExpiredUsers(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	email := uniqueEmail("ubexp")
	env.registerAndLogin(t, email, "ubexp_"+shortID())

	user, err := env.queries.GetUserByEmail(ctx, email)
	require.NoError(t, err)

	// Бан с истёкшим сроком.
	_, err = env.pgPool.Exec(ctx,
		`UPDATE users SET is_banned=true, ban_reason='expired',
		 ban_expires_at = now() - interval '1 minute' WHERE id = $1`, user.ID)
	require.NoError(t, err)

	// Дёргаем запрос вручную (воркер вызывал бы его на тике).
	err = env.queries.UnbanExpiredUsers(ctx)
	require.NoError(t, err)

	// Проверяем, что бан снят.
	var isBanned bool
	var banReason *string
	var banExpiresAt *time.Time
	err = env.pgPool.QueryRow(ctx,
		`SELECT is_banned, ban_reason, ban_expires_at FROM users WHERE id = $1`, user.ID).
		Scan(&isBanned, &banReason, &banExpiresAt)
	require.NoError(t, err)
	require.False(t, isBanned)
	require.Nil(t, banReason)
	require.Nil(t, banExpiresAt)
}
