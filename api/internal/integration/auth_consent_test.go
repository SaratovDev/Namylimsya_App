package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterRequiresConsent(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	base := func() map[string]any {
		return map[string]any{
			"email":    uniqueEmail("consent"),
			"password": "secret123",
			"username": "cons_" + shortID(),
			"city":     "Саратов",
		}
	}

	// 1. Без consent_accepted → 400
	body := base()
	body["age_confirmed"] = true
	body["consent_version"] = "1.0"
	r := env.srv.do(t, httpReq{method: "POST", path: "/api/v1/auth/register", body: body})
	require.Equal(t, 400, r.Status, r.Raw)
	require.Contains(t, r.Body["message"], "согласие")

	// 2. Без age_confirmed → 400
	body = base()
	body["consent_accepted"] = true
	body["consent_version"] = "1.0"
	r = env.srv.do(t, httpReq{method: "POST", path: "/api/v1/auth/register", body: body})
	require.Equal(t, 400, r.Status, r.Raw)
	require.Contains(t, r.Body["message"], "18")

	// 3. Устаревшая версия → 400
	body = base()
	body["consent_accepted"] = true
	body["age_confirmed"] = true
	body["consent_version"] = "0.9"
	r = env.srv.do(t, httpReq{method: "POST", path: "/api/v1/auth/register", body: body})
	require.Equal(t, 400, r.Status, r.Raw)
	require.Contains(t, r.Body["message"], "устарела")

	// 4. Всё передано → 201, в БД consent сохранён
	email := uniqueEmail("consent_ok")
	body = map[string]any{
		"email":            email,
		"password":         "secret123",
		"username":         "consok_" + shortID(),
		"city":             "Саратов",
		"consent_accepted": true,
		"age_confirmed":    true,
		"consent_version":  "1.0",
	}
	r = env.srv.do(t, httpReq{method: "POST", path: "/api/v1/auth/register", body: body})
	require.Equal(t, 201, r.Status, r.Raw)

	// Проверяем БД
	var consentAt, ageConfirmedAt *string
	var consentVersion *string
	err := env.pgPool.QueryRow(context.Background(),
		`SELECT consent_at::text, consent_version, age_confirmed_at::text
		 FROM users WHERE email = $1`, email).
		Scan(&consentAt, &consentVersion, &ageConfirmedAt)
	require.NoError(t, err)
	require.NotNil(t, consentAt, "consent_at must be saved")
	require.NotNil(t, consentVersion, "consent_version must be saved")
	require.Equal(t, "1.0", *consentVersion)
	require.NotNil(t, ageConfirmedAt, "age_confirmed_at must be saved")
}
