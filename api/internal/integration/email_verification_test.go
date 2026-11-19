package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Свежий пользователь — email_verified=false; невалидный токен — 400.
func TestEmailVerificationBasics(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("ev_u"), "evu_"+shortID())

	// /users/me отдаёт email_verified=false для нового юзера.
	me := env.srv.do(t, httpReq{method: "GET", path: "/api/v1/users/me", token: token})
	require.Equal(t, 200, me.Status, me.Raw)
	require.False(t, me.Body["email_verified"].(bool), "новый аккаунт должен быть без подтверждённого email")

	// Невалидный/пустой токен — 400.
	bad := env.srv.do(t, httpReq{method: "GET", path: "/api/v1/auth/verify-email?token=bogus"})
	require.Equal(t, 400, bad.Status, bad.Raw)
}
