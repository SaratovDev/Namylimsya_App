package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccessTokenRefreshFlow(t *testing.T) {
	// Короткий TTL — чтобы дождаться истечения за секунды, а не за 15 минут.
	// t.Setenv имеет приоритет над .env (godotenv.Load не переопределяет).
	t.Setenv("JWT_ACCESS_TTL", "1s")

	env := newTestEnv(t)
	defer env.cleanup()

	email := uniqueEmail("refresh")
	username := "ref_" + shortID()

	// Регистрируем (создаём пользователя), затем логинимся.
	regResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/register",
		body: map[string]any{
			"email": email, "password": "secret123", "username": username,
			"consent_accepted": true,
			"age_confirmed":    true,
			"consent_version":  "1.0",
		},
	})
	require.Equal(t, 201, regResp.Status, regResp.Raw)

	accessToken, cookies := env.loginRaw(t, email, "secret123")
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, cookies)

	// 1. Свежий access работает
	me1 := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me", token: accessToken,
	})
	require.Equal(t, 200, me1.Status, me1.Raw)

	// 2. Ждём истечения access (TTL=1s)
	time.Sleep(1500 * time.Millisecond)

	meExpired := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me", token: accessToken,
	})
	require.Equal(t, 401, meExpired.Status, "access must be expired, got %s", meExpired.Raw)

	// 3. Refresh по cookie
	rc := findCookie(cookies, "refresh_token")
	require.NotEmpty(t, rc)
	refreshCookie := cookieKV(rc)

	refreshResp := env.srv.doRaw(t, httpReq{
		method: "POST", path: "/api/v1/auth/refresh",
		cookies: refreshCookie,
	})
	require.Equal(t, 200, refreshResp.StatusCode)
	defer refreshResp.Body.Close()

	var body map[string]any
	require.NoError(t, json.NewDecoder(refreshResp.Body).Decode(&body))
	newToken, _ := body["access_token"].(string)
	require.NotEmpty(t, newToken)
	require.NotEqual(t, accessToken, newToken, "refresh must issue a new access token")

	// 4. Новый access работает
	me2 := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me", token: newToken,
	})
	require.Equal(t, 200, me2.Status, me2.Raw)

	// 5. Старый refresh (из login) должен быть отозван ротацией — повторный
	// refresh с тем же cookie даёт 401.
	refreshAgain := env.srv.doRaw(t, httpReq{
		method: "POST", path: "/api/v1/auth/refresh",
		cookies: refreshCookie,
	})
	require.Equal(t, 401, refreshAgain.StatusCode, "refresh token must be rotated")
	defer refreshAgain.Body.Close()
}
