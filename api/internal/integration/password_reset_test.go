package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPasswordResetFlow(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	email := uniqueEmail("pwreset")
	env.registerAndLogin(t, email, "pwreset_"+shortID())

	// 1. Запрос сброса.
	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/password-reset-request",
		body: map[string]any{"email": email},
	})
	require.Equal(t, 200, r.Status, r.Raw)

	// 2. Код в Redis.
	code, err := env.rdb.Get(ctx, "pwreset:code:"+email).Result()
	require.NoError(t, err)
	require.Len(t, code, 6)

	// 3. Сброс с неверным кодом — 400.
	rBad := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/password-reset",
		body: map[string]any{
			"email": email, "code": "000000", "new_password": "newpassword123",
		},
	})
	require.Equal(t, 400, rBad.Status, rBad.Raw)

	// 4. Сброс с верным кодом — 200.
	rOk := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/password-reset",
		body: map[string]any{
			"email": email, "code": code, "new_password": "newpassword123",
		},
	})
	require.Equal(t, 200, rOk.Status, rOk.Raw)

	// 5. Старый пароль не работает.
	oldLogin := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/login",
		body: map[string]any{"email": email, "password": "secret123"},
	})
	require.Equal(t, 401, oldLogin.Status)

	// 6. Новый — работает.
	newLogin := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/login",
		body: map[string]any{"email": email, "password": "newpassword123"},
	})
	require.Equal(t, 200, newLogin.Status, newLogin.Raw)
	require.NotEmpty(t, newLogin.Body["access_token"])

	// 7. Код после использования удалён.
	_, err = env.rdb.Get(ctx, "pwreset:code:"+email).Result()
	require.Error(t, err) // redis.Nil
}

func TestPasswordResetEnumerationProtection(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Несуществующий email — 200 (не палим).
	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/password-reset-request",
		body: map[string]any{"email": "nonexistent-" + shortID() + "@example.com"},
	})
	require.Equal(t, 200, r.Status, r.Raw)
}

func TestPasswordResetThrottle(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	email := uniqueEmail("pwthrottle")
	env.registerAndLogin(t, email, "pwthr_"+shortID())

	// 1-й запрос — ок.
	r1 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/password-reset-request",
		body: map[string]any{"email": email},
	})
	require.Equal(t, 200, r1.Status)

	// Сразу 2-й — тоже 200 (не ошибка), но код НЕ перезаписан.
	oldCode, _ := env.rdb.Get(ctx, "pwreset:code:"+email).Result()

	time.Sleep(100 * time.Millisecond)
	r2 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/password-reset-request",
		body: map[string]any{"email": email},
	})
	require.Equal(t, 200, r2.Status)

	newCode, _ := env.rdb.Get(ctx, "pwreset:code:"+email).Result()
	require.Equal(t, oldCode, newCode, "2-й запрос в течение 60 сек не должен менять код")
}
