package integration

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
)

// dialWS открывает WS-соединение с токеном в Sec-WebSocket-Protocol
// (["bearer", token]). Path без ?token= — токен больше не в query.
func dialWS(t *testing.T, env *testEnv, path, token string) (*websocket.Conn, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(env.srv)
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + path
	header := map[string][]string{
		"Sec-WebSocket-Protocol": {"bearer", token},
	}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	require.NoError(t, err, "handshake должен пройти (subprotocol + token)")
	return conn, srv
}

// expectCloseCode читает сообщение и ожидает Close-frame с заданным кодом.
func expectCloseCode(t *testing.T, conn *websocket.Conn, wantCode int) {
	t.Helper()
	_, _, err := conn.ReadMessage()
	require.Error(t, err, "ожидали close-frame, а не сообщение")

	var closeErr *websocket.CloseError
	require.True(t, errors.As(err, &closeErr),
		"ожидали *websocket.CloseError, got %T: %v", err, err)
	require.Equal(t, wantCode, closeErr.Code,
		"close-code: got %d (%q), want %d", closeErr.Code, closeErr.Text, wantCode)
}

func TestWSHandshakeRejectsBannedUser(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	adminEmail := uniqueEmail("wsb_a")
	userEmail := uniqueEmail("wsb_u")

	env.registerAndLogin(t, adminEmail, "wsba_"+shortID())
	userToken := env.registerAndLogin(t, userEmail, "wsbu_"+shortID())

	adminUser, _ := env.queries.GetUserByEmail(ctx, adminEmail)
	_, _ = env.queries.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: adminUser.ID, Role: "admin",
	})
	adminToken, _ := env.loginRaw(t, adminEmail, "secret123")

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: adminToken,
		body: map[string]any{
			"title": "WS ban", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	applyResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications",
		token: userToken,
	})
	require.Equal(t, 201, applyResp.Status, applyResp.Raw)
	appID := applyResp.Body["id"].(string)
	env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID + "/approve",
		token: adminToken,
	})

	targetUser, _ := env.queries.GetUserByEmail(ctx, userEmail)
	reason := "test ban"
	_, err := env.queries.AdminBanUser(ctx, dbgen.AdminBanUserParams{
		ID:        targetUser.ID,
		IsBanned:  true,
		BanReason: &reason,
	})
	require.NoError(t, err)
	require.NoError(t, env.rdb.Set(ctx, "banned:"+targetUser.ID.String(), "1", time.Minute).Err())

	conn, _ := dialWS(t, env, "/ws/events/"+eventID, userToken)
	defer conn.Close()
	expectCloseCode(t, conn, 4403)
}

func TestWSHandshakeRejectsInvalidToken(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("wsi_o"), "wsio_"+shortID())
	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "WS invalid token", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	conn, _ := dialWS(t, env, "/ws/events/"+eventID, "not-a-valid-jwt")
	defer conn.Close()
	expectCloseCode(t, conn, 4401)
}

func TestWSHandshakeRejectsForbidden(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("wsf_o"), "wsfo_"+shortID())
	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "WS forbidden", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	strangerToken := env.registerAndLogin(t, uniqueEmail("wsf_s"), "wsfs_"+shortID())

	conn, _ := dialWS(t, env, "/ws/events/"+eventID, strangerToken)
	defer conn.Close()
	expectCloseCode(t, conn, 4403)
}

func TestWSHandshakeRejectsInvalidEventID(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("wsb_id"), "wsbid_"+shortID())

	conn, _ := dialWS(t, env, "/ws/events/not-a-uuid", token)
	defer conn.Close()
	expectCloseCode(t, conn, 4400)
}
