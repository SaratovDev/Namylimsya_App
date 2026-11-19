package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
)

func TestBanFlow(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	// 1. Регистрируем трёх пользователей.
	adminEmail := uniqueEmail("ban_admin")
	targetEmail := uniqueEmail("ban_target")
	otherEmail := uniqueEmail("ban_other")

	adminToken := env.registerAndLogin(t, adminEmail, "adm_"+shortID())
	targetToken := env.registerAndLogin(t, targetEmail, "tgt_"+shortID())
	otherToken := env.registerAndLogin(t, otherEmail, "oth_"+shortID())

	// 2. Назначаем admin'ом через queries (регистрация даёт role=user).
	adminUser, err := env.queries.GetUserByEmail(ctx, adminEmail)
	require.NoError(t, err)
	_, err = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: adminUser.ID, Role: "admin",
	})
	require.NoError(t, err)

	// ВАЖНО: access-токен admin'а был выдан с role=user (до UPDATE).
	// Перелогиниваемся, чтобы JWT содержал role=admin.
	adminToken, _ = env.loginRaw(t, adminEmail, "secret123")

	// 3. Получаем target user_id.
	targetUser, err := env.queries.GetUserByEmail(ctx, targetEmail)
	require.NoError(t, err)

	// 4. ДО бана target ходит на /users/me.
	meBefore := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me", token: targetToken,
	})
	require.Equal(t, 200, meBefore.Status, "target must access /users/me before ban")

	// 5. Refresh-ключи target'а в Redis есть.
	pattern := fmt.Sprintf("refresh:%s:*", targetUser.ID.String())
	keysBefore, err := env.rdb.Keys(ctx, pattern).Result()
	require.NoError(t, err)
	require.NotEmpty(t, keysBefore, "target must have refresh keys before ban")

	// 6. Баним target от админа.
	banResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + targetUser.ID.String() + "/ban",
		token: adminToken,
		body:  map[string]any{"banned": true, "reason": "test ban"},
	})
	require.Equal(t, 200, banResp.Status, banResp.Raw)
	require.Equal(t, true, banResp.Body["is_banned"])

	// 7. Refresh-ключи ревокированы.
	keysAfter, err := env.rdb.Keys(ctx, pattern).Result()
	require.NoError(t, err)
	require.Empty(t, keysAfter, "refresh keys must be revoked after ban")

	// 8. Забаненный не может логиниться.
	loginResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/login",
		body: map[string]any{"email": targetEmail, "password": "secret123"},
	})
	require.Equal(t, 403, loginResp.Status, loginResp.Raw)
	require.Equal(t, "banned", loginResp.Body["error"])

	// 9. Забаненный с уже выданным access-токеном получает 403.
	meBanned := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me", token: targetToken,
	})
	require.Equal(t, 403, meBanned.Status, meBanned.Raw)
	require.Equal(t, "banned", meBanned.Body["error"])

	// 10. Другой не забанен.
	otherMe := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me", token: otherToken,
	})
	require.Equal(t, 200, otherMe.Status)

	// 11. Разбан.
	unbanResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/admin/users/" + targetUser.ID.String() + "/ban",
		token: adminToken,
		body:  map[string]any{"banned": false},
	})
	require.Equal(t, 200, unbanResp.Status, unbanResp.Raw)

	// 12. После разбана target снова логинится.
	_, _ = env.loginRaw(t, targetEmail, "secret123")

	// 13. И старый access-токен снова работает (Redis-флаг banned: снят).
	meAfterUnban := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me", token: targetToken,
	})
	require.Equal(t, 200, meAfterUnban.Status, meAfterUnban.Raw)

	_ = uuid.Nil // uuid импортирован для возможного расширения
}

func TestRefreshCookieAttributes(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	email := uniqueEmail("cookie_user")
	env.registerAndLogin(t, email, "ck_"+shortID())

	// 1. Login → Set-Cookie.
	_, cookies := env.loginRaw(t, email, "secret123")
	require.NotEmpty(t, cookies)

	rc := findCookie(cookies, "refresh_token")
	require.NotEmpty(t, rc, "refresh_token must be present")

	require.Contains(t, rc, "Path=/api/v1/auth")
	require.Contains(t, rc, "HttpOnly")
	require.Contains(t, rc, "SameSite=Lax")
	require.Contains(t, rc, "Max-Age=2592000", "Max-Age must be 30 days")
	require.NotContains(t, strings.ToLower(rc), "secure",
		"Secure must be absent in dev")

	refreshCookie := cookieKV(rc) // "refresh_token=eyJ..."
	require.NotEmpty(t, refreshCookie)

	// 2. Refresh → новый Set-Cookie. Передаём cookie из логина.
	refreshResp := env.srv.doRaw(t, httpReq{
		method: "POST", path: "/api/v1/auth/refresh",
		cookies: refreshCookie,
	})
	require.Equal(t, 200, refreshResp.StatusCode, "refresh must succeed with valid cookie")
	defer refreshResp.Body.Close()

	rcRefresh := findCookie(refreshResp.Header.Values("Set-Cookie"), "refresh_token")
	require.NotEmpty(t, rcRefresh)
	require.Contains(t, rcRefresh, "Path=/api/v1/auth")
	require.Contains(t, rcRefresh, "Max-Age=2592000")

	// 3. Logout → Max-Age=0. Передаём актуальную (после ротации) cookie.
	newRefreshCookie := cookieKV(rcRefresh)
	require.NotEmpty(t, newRefreshCookie)

	logoutResp := env.srv.doRaw(t, httpReq{
		method: "POST", path: "/api/v1/auth/logout",
		cookies: newRefreshCookie,
	})
	require.Equal(t, 204, logoutResp.StatusCode)
	defer logoutResp.Body.Close()

	rcLogout := findCookie(logoutResp.Header.Values("Set-Cookie"), "refresh_token")
	require.NotEmpty(t, rcLogout)
	require.Contains(t, rcLogout, "Max-Age=0")
	require.Contains(t, rcLogout, "Path=/api/v1/auth")
}
