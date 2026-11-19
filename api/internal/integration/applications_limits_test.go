package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Реактивация cancelled заявки не обходит лимит 3 активных.
func TestReactivateDoesNotBypassLimit(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	userToken := env.registerAndLogin(t, uniqueEmail("rbl_u"), "rbl_u_"+shortID())
	orgToken := env.registerAndLogin(t, uniqueEmail("rbl_o"), "rbl_o_"+shortID())
	org2Token := env.registerAndLogin(t, uniqueEmail("rbl_o2"), "rbl_o2_"+shortID())

	// 4 события (2 от каждого орга — у них лимит 2 активных)
	eventIDs := []string{}
	for i := 0; i < 2; i++ {
		resp := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events", token: orgToken,
			body: map[string]any{
				"title": "E1", "category": "прогулки",
				"starts_at": futureTime(time.Duration(24+i) * time.Hour).Format(time.RFC3339),
				"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			},
		})
		require.Equal(t, 201, resp.Status, resp.Raw)
		eventIDs = append(eventIDs, resp.Body["id"].(string))
	}
	for i := 0; i < 2; i++ {
		resp := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events", token: org2Token,
			body: map[string]any{
				"title": "E2", "category": "спорт",
				"starts_at": futureTime(time.Duration(24+i) * time.Hour).Format(time.RFC3339),
				"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			},
		})
		require.Equal(t, 201, resp.Status, resp.Raw)
		eventIDs = append(eventIDs, resp.Body["id"].(string))
	}

	// 3 активные заявки
	appIDs := []string{}
	for i := 0; i < 3; i++ {
		resp := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events/" + eventIDs[i] + "/applications", token: userToken,
		})
		require.Equal(t, 201, resp.Status, resp.Raw)
		appIDs = append(appIDs, resp.Body["id"].(string))
	}

	// Отменяем первую — теперь активны 2
	cancelResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/applications/" + appIDs[0] + "/cancel", token: userToken,
		body: map[string]any{"reason": "test"},
	})
	require.Equal(t, 200, cancelResp.Status, cancelResp.Raw)

	// Реактивируем cancelled на eventIDs[3] — лимит 3, слот есть → ок
	react := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventIDs[3] + "/applications", token: userToken,
	})
	require.Equal(t, 201, react.Status, react.Raw)
	require.Equal(t, "pending", react.Body["status"])

	// Реактивируем appIDs[0] (cancelled) на eventIDs[0] — теперь активны 4 → 400
	react2 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventIDs[0] + "/applications", token: userToken,
	})
	require.Equal(t, 400, react2.Status, react2.Raw)
	require.Contains(t, react2.Body["message"], "Больше трех заявок")
}

// Заявка на прошедшее событие отвергается.
func TestApplyToPastEventRejected(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("past_o"), "past_o_"+shortID())
	userToken := env.registerAndLogin(t, uniqueEmail("past_u"), "past_u_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Past", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// Симулируем прошедшее событие (в обход API)
	_, err := env.pgPool.Exec(context.Background(),
		"UPDATE events SET starts_at = now() - interval '1 hour' WHERE id = $1", eventID)
	require.NoError(t, err)

	applyResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: userToken,
	})
	require.Equal(t, 400, applyResp.Status, applyResp.Raw)
	require.Contains(t, applyResp.Body["message"], "будущем")
}

// Approve сверх participant_limit отвергается.
func TestApproveOverParticipantLimit(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("aol_o"), "aol_o_"+shortID())
	u1 := env.registerAndLogin(t, uniqueEmail("aol_u1"), "aol_u1_"+shortID())
	u2 := env.registerAndLogin(t, uniqueEmail("aol_u2"), "aol_u2_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Limit", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			"participant_limit": 1,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	app1 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: u1,
	})
	require.Equal(t, 201, app1.Status)
	appID1 := app1.Body["id"].(string)

	app2 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: u2,
	})
	require.Equal(t, 201, app2.Status)
	appID2 := app2.Body["id"].(string)

	// Первый — ок
	r1 := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID1 + "/approve", token: orgToken,
	})
	require.Equal(t, 200, r1.Status, r1.Raw)

	// Второй — 400 (event full)
	r2 := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID2 + "/approve", token: orgToken,
	})
	require.Equal(t, 400, r2.Status, r2.Raw)
	require.Contains(t, r2.Body["message"], "хватает участников")
}

// Отмена события каскадно отменяет pending и approved заявки.
func TestCancelEventCancelsApplications(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("cc_o"), "cc_o_"+shortID())
	u1 := env.registerAndLogin(t, uniqueEmail("cc_u1"), "cc_u1_"+shortID())
	u2 := env.registerAndLogin(t, uniqueEmail("cc_u2"), "cc_u2_"+shortID())

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Cancel", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			"participant_limit": 10,
		},
	})
	require.Equal(t, 201, resp.Status)
	eventID := resp.Body["id"].(string)

	// u1 — approved
	app1 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: u1,
	})
	require.Equal(t, 201, app1.Status)
	appID1 := app1.Body["id"].(string)
	env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID1 + "/approve", token: orgToken,
	})

	// u2 — pending
	app2 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: u2,
	})
	require.Equal(t, 201, app2.Status)
	appID2 := app2.Body["id"].(string)

	// Отменяем событие
	cancelResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/cancel", token: orgToken,
		body: map[string]any{"reason": "Планы изменились"},
	})
	require.Equal(t, 200, cancelResp.Status, cancelResp.Raw)

	// Обе заявки — cancelled
	for _, appID := range []string{appID1, appID2} {
		var status string
		var reason *string
		err := env.pgPool.QueryRow(context.Background(),
			"SELECT status, cancel_reason FROM applications WHERE id = $1", appID).
			Scan(&status, &reason)
		require.NoError(t, err)
		require.Equal(t, "cancelled", status, "app %s must be cancelled", appID)
		require.NotNil(t, reason)
		require.Equal(t, "Планы изменились", *reason)
	}
}

// Отмена события освобождает слоты пользователя в лимите 3.
func TestCancelEventFreesApplicationSlots(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("fs_o"), "fs_o_"+shortID())
	org2Token := env.registerAndLogin(t, uniqueEmail("fs_o2"), "fs_o2_"+shortID())
	userToken := env.registerAndLogin(t, uniqueEmail("fs_u"), "fs_u_"+shortID())

	// 3 события: 2 от первого орга, 1 от второго (лимит орга — 2)
	eventIDs := []string{}
	for i := 0; i < 2; i++ {
		resp := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events", token: orgToken,
			body: map[string]any{
				"title": "E", "category": "прогулки",
				"starts_at": futureTime(time.Duration(24+i) * time.Hour).Format(time.RFC3339),
				"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			},
		})
		require.Equal(t, 201, resp.Status, resp.Raw)
		eventIDs = append(eventIDs, resp.Body["id"].(string))
	}
	resp3 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: org2Token,
		body: map[string]any{
			"title": "E3", "category": "спорт",
			"starts_at": futureTime(26 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp3.Status, resp3.Raw)
	eventIDs = append(eventIDs, resp3.Body["id"].(string))

	// 3 заявки — лимит исчерпан
	for _, eid := range eventIDs {
		resp := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events/" + eid + "/applications", token: userToken,
		})
		require.Equal(t, 201, resp.Status, resp.Raw)
	}

	// Отменяем одно событие → один слот освобождается
	env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventIDs[0] + "/cancel", token: orgToken,
		body: map[string]any{"reason": "test"},
	})

	// Создаём 4-е событие другим оргом и подаём — 201 (слот освободился)
	// Создаём 4-е событие (у org2 только 1 активное, лимит 2) и подаём — 201
	resp4 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: org2Token,
		body: map[string]any{
			"title": "E4", "category": "спорт",
			"starts_at": futureTime(48 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp4.Status, resp4.Raw)
	eid4 := resp4.Body["id"].(string)

	applyResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eid4 + "/applications", token: userToken,
	})
	require.Equal(t, 201, applyResp.Status, applyResp.Raw)
}
