package integration

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// Subprotocol "bearer" + валидный токен → 101 + subprotocol "bearer".
func TestWSSubprotocolAcceptsBearer(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("wsp_o"), "wspo_"+shortID())
	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "WS proto", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	conn, _ := dialWS(t, env, "/ws/events/"+eventID, orgToken)
	defer conn.Close()

	require.Equal(t, "bearer", conn.Subprotocol(),
		"сервер должен согласовать subprotocol 'bearer'")
}

// Без subprotocol "bearer" Upgrade вернёт 400 (клиент не пройдёт handshake).
func TestWSNoSubprotocolRejected(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("wsn_o"), "wsno_"+shortID())
	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "WS no proto", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	srv := httptest.NewServer(env.srv)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/events/" + eventID
	// Без Sec-WebSocket-Protocol.
	_, resp2, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.Error(t, err, "handshake без subprotocol должен провалиться")
	if resp2 != nil {
		require.Equal(t, 400, resp2.StatusCode)
	}
}

// Legacy ?token= больше не работает: клиент с query-токеном и без
// subprotocol получает 400.
func TestWSLegacyQueryTokenRejected(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("wsl_o"), "wslo_"+shortID())
	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "WS legacy", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	srv := httptest.NewServer(env.srv)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") +
		"/ws/events/" + eventID + "?token=" + orgToken
	_, resp2, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.Error(t, err, "legacy ?token= без subprotocol — 400")
	if resp2 != nil {
		require.Equal(t, 400, resp2.StatusCode)
	}
}

// LRU: 4-е соединение у одного юзера закрывает старейшее (4408),
// новое живёт. Cap = 3.
func TestWSReplacedLRU(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("wsr_o"), "wsro_"+shortID())
	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "WS LRU", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// Открываем 3 соединения (cap = 3).
	c1, _ := dialWS(t, env, "/ws/events/"+eventID, orgToken)
	defer c1.Close()
	time.Sleep(10 * time.Millisecond)

	c2, _ := dialWS(t, env, "/ws/events/"+eventID, orgToken)
	defer c2.Close()
	time.Sleep(10 * time.Millisecond)

	c3, _ := dialWS(t, env, "/ws/events/"+eventID, orgToken)
	defer c3.Close()
	time.Sleep(10 * time.Millisecond)

	// 4-е — эвиктит c1.
	c4, _ := dialWS(t, env, "/ws/events/"+eventID, orgToken)
	defer c4.Close()

	// c1 получает 4408.
	expectCloseCode(t, c1, 4408)

	// c2 и c3 живы (можно писать).
	_ = c2.SetWriteDeadline(time.Now().Add(time.Second))
	require.NoError(t, c2.WriteMessage(websocket.TextMessage, []byte(`{"text":"still alive"}`)))
	_ = c3.SetWriteDeadline(time.Now().Add(time.Second))
	require.NoError(t, c3.WriteMessage(websocket.TextMessage, []byte(`{"text":"still alive"}`)))
}
