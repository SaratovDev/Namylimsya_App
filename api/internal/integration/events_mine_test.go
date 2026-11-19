package integration

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMyOrganizedEvents(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("mine_org"), "mo_"+shortID())
	otherToken := env.registerAndLogin(t, uniqueEmail("mine_oth"), "moth_"+shortID())

	uniqueCity := "Саратов-" + shortID()

	// Орг создаёт 2 активных события.
	ids := []string{}
	for i := 0; i < 2; i++ {
		resp := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events", token: orgToken,
			body: map[string]any{
				"title": "Mine", "category": "прогулки",
				"starts_at": futureTime(time.Duration(24+i) * time.Hour).Format(time.RFC3339),
				"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
			},
		})
		require.Equal(t, 201, resp.Status, resp.Raw)
		ids = append(ids, resp.Body["id"].(string))
	}

	// Отменяем первое.
	cancelResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + ids[0] + "/cancel", token: orgToken,
		body: map[string]any{"reason": "test"},
	})
	require.Equal(t, 200, cancelResp.Status, cancelResp.Raw)

	// GET /users/me/events возвращает оба (active + cancelled).
	mineResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me/events", token: orgToken,
	})
	require.Equal(t, 200, mineResp.Status, mineResp.Raw)
	events := mineResp.Body["events"].([]any)
	require.Len(t, events, 2)

	statuses := map[string]bool{}
	for _, e := range events {
		m := e.(map[string]any)
		require.Equal(t, true, m["can_see_exact_location"], "organizer always sees exact")
		statuses[m["status"].(string)] = true
	}
	require.True(t, statuses["active"])
	require.True(t, statuses["cancelled"])

	// Другой пользователь не видит чужие.
	otherResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me/events", token: otherToken,
	})
	require.Equal(t, 200, otherResp.Status, otherResp.Raw)
	require.Empty(t, otherResp.Body["events"])

	// Гость — 401.
	guestResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/users/me/events",
	})
	require.Equal(t, 401, guestResp.Status)

	// Фильтр фида по city — sanity: в активном фиде только одно (не отменённое).
	feedResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events?city=" + url.QueryEscape(uniqueCity),
	})
	require.Equal(t, 200, feedResp.Status, feedResp.Raw)
	feedEvents := feedResp.Body["events"].([]any)
	require.Len(t, feedEvents, 1, "active feed must contain exactly 1 event")
	require.Equal(t, ids[1], feedEvents[0].(map[string]any)["id"])
	require.Equal(t, "active", feedEvents[0].(map[string]any)["status"])
}

// GetByID возвращает user_application_id для участника с заявкой.
func TestEventDetailIncludesApplicationID(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("ai_o"), "aio_"+shortID())
	userToken := env.registerAndLogin(t, uniqueEmail("ai_u"), "aiu_"+shortID())
	strangerToken := env.registerAndLogin(t, uniqueEmail("ai_s"), "ais_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "AI", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// Applicant подаёт заявку.
	applyResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: userToken,
	})
	require.Equal(t, 201, applyResp.Status, applyResp.Raw)
	appID := applyResp.Body["id"].(string)

	// 1. Участник с заявкой: user_application_id = id заявки.
	detail := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: userToken,
	})
	require.Equal(t, 200, detail.Status, detail.Raw)
	require.Equal(t, appID, detail.Body["user_application_id"])
	require.Equal(t, "pending", detail.Body["user_application_status"])

	// 2. Организатор: user_application_id = null (он не подаёт заявку).
	orgDetail := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: orgToken,
	})
	require.Equal(t, 200, orgDetail.Status, orgDetail.Raw)
	require.Nil(t, orgDetail.Body["user_application_id"])
	require.Equal(t, "", orgDetail.Body["user_application_status"])

	// 3. Гость: user_application_id = null.
	guestDetail := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID,
	})
	require.Equal(t, 200, guestDetail.Status, guestDetail.Raw)
	require.Nil(t, guestDetail.Body["user_application_id"])

	// 4. Посторонний (без заявки): null.
	strangerDetail := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: strangerToken,
	})
	require.Equal(t, 200, strangerDetail.Status, strangerDetail.Raw)
	require.Nil(t, strangerDetail.Body["user_application_id"])
}
