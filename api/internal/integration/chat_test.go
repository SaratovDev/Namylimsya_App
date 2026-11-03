package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChatREST(t *testing.T) {
	env := newTestEnv(t)

	orgToken := env.registerAndLogin(t, uniqueEmail("chat_org"), "chat_org_"+shortID())
	appToken := env.registerAndLogin(t, uniqueEmail("chat_app"), "chat_app_"+shortID())
	strangerToken := env.registerAndLogin(t, uniqueEmail("chat_str"), "chat_str_"+shortID())

	// Создаём событие от organizer
	eventResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Chat test", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, eventResp.Status, eventResp.Raw)
	eventID := eventResp.Body["id"].(string)

	// Applicant подаёт заявку
	applyResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: appToken,
	})
	require.Equal(t, 201, applyResp.Status, applyResp.Raw)
	appID := applyResp.Body["id"].(string)

	// Organizer одобряет
	approveResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID + "/approve", token: orgToken,
	})
	require.Equal(t, 200, approveResp.Status, approveResp.Raw)

	// 1) Organizer отправляет сообщение
	sendResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/messages", token: orgToken,
		body: map[string]any{"text": "Привет!"},
	})
	require.Equal(t, 201, sendResp.Status, sendResp.Raw)

	// 2) Applicant видит историю
	histResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: appToken,
	})
	require.Equal(t, 200, histResp.Status, histResp.Raw)
	msgs := histResp.Body["messages"].([]any)
	require.Len(t, msgs, 1)
	require.Equal(t, "Привет!", msgs[0].(map[string]any)["text"])

	// 3) Stranger — 403
	histStr := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/messages", token: strangerToken,
	})
	require.Equal(t, 403, histStr.Status)

	// 4) Отправить пустое — 400
	emptyResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/messages", token: orgToken,
		body: map[string]any{"text": "   "},
	})
	require.Equal(t, 400, emptyResp.Status)

	// 5) Отправить слишком длинное (>1000) — 400
	longText := ""
	for i := 0; i < 1001; i++ {
		longText += "a"
	}
	longResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/messages", token: orgToken,
		body: map[string]any{"text": longText},
	})
	require.Equal(t, 400, longResp.Status)

	// 6) Applicant тоже может писать (после approve)
	appSendResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/messages", token: appToken,
		body: map[string]any{"text": "Привет от applicant"},
	})
	require.Equal(t, 201, appSendResp.Status, appSendResp.Raw)
}
