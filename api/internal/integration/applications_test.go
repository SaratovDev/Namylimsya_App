package integration

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApplicationsFlow(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// 1. Пользователи
	organizerEmail := uniqueEmail("organizer")
	applicantEmail := uniqueEmail("applicant")
	strangerEmail := uniqueEmail("stranger")

	organizerToken := env.registerAndLogin(t, organizerEmail, "org_"+shortID())
	applicantToken := env.registerAndLogin(t, applicantEmail, "app_"+shortID())
	strangerToken := env.registerAndLogin(t, strangerEmail, "str_"+shortID())

	// 2. Создаём событие от organizer
	eventResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: organizerToken,
		body: map[string]any{
			"title": "Test event", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			"hide_exact_location": false, "participant_limit": 10,
		},
	})
	require.Equal(t, 201, eventResp.Status, eventResp.Raw)
	eventID := eventResp.Body["id"].(string)

	// 3. Тест 1: applicant подаёт заявку
	applyResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: applicantToken,
	})
	require.Equal(t, 201, applyResp.Status, applyResp.Raw)
	require.Equal(t, "pending", applyResp.Body["status"])
	appID := applyResp.Body["id"].(string)

	// 4. Тест 2: повторная — 400
	dupResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: applicantToken,
	})
	require.Equal(t, 400, dupResp.Status)
	require.Contains(t, dupResp.Body["message"], "уже отправлена заяка")
	// 5. Тест 3: на своё событие — 400
	ownResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: organizerToken,
	})
	require.Equal(t, 400, ownResp.Status)
	require.Contains(t, ownResp.Body["message"], "зачем на него заявляться")
	// 6. Тест 4a: организатор видит заявки — 200
	listResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/applications", token: organizerToken,
	})
	require.Equal(t, 200, listResp.Status, listResp.Raw)

	// 7. Тест 4b: посторонний — 403
	strangerList := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/applications", token: strangerToken,
	})
	require.Equal(t, 403, strangerList.Status)

	// 8. Тест 5: одобрение — 200
	approveResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID + "/approve", token: organizerToken,
	})
	require.Equal(t, 200, approveResp.Status, approveResp.Raw)
	require.Equal(t, "approved", approveResp.Body["status"])

	// 9. Тест 8: отмена участником — 200
	cancelResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/applications/" + appID + "/cancel", token: applicantToken,
		body: map[string]any{"reason": "Планы изменились"},
	})
	require.Equal(t, 200, cancelResp.Status, cancelResp.Raw)
	require.Equal(t, "cancelled", cancelResp.Body["status"])
	require.Equal(t, "Планы изменились", cancelResp.Body["cancel_reason"])

	// 10. Тест 6: stranger пытается одобрить чужую заявку — нужна новая заявка
	apply2 := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: strangerToken,
	})
	require.Equal(t, 201, apply2.Status, apply2.Raw)
	app2ID := apply2.Body["id"].(string)

	notOwnerResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + app2ID + "/approve", token: applicantToken,
	})
	require.Equal(t, 403, notOwnerResp.Status)

	// 11. Тест 7: организатор отклоняет — 200
	rejectResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + app2ID + "/reject", token: organizerToken,
	})
	require.Equal(t, 200, rejectResp.Status, rejectResp.Raw)
	require.Equal(t, "rejected", rejectResp.Body["status"])
}

func TestHiddenLocation(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	organizerEmail := uniqueEmail("org_hidden")
	applicantEmail := uniqueEmail("app_hidden")

	organizerToken := env.registerAndLogin(t, organizerEmail, "oh_"+shortID())
	applicantToken := env.registerAndLogin(t, applicantEmail, "ah_"+shortID())

	// Создаём событие с hide_exact_location: true
	eventResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: organizerToken,
		body: map[string]any{
			"title": "Hidden event", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "address": "Точный адрес",
			"lat": 51.53, "lng": 46.03,
			"hide_exact_location": true,
		},
	})
	require.Equal(t, 201, eventResp.Status, eventResp.Raw)
	eventID := eventResp.Body["id"].(string)

	// Гость — не видит адрес
	guestResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID,
	})
	require.Equal(t, 200, guestResp.Status, guestResp.Raw)
	require.Equal(t, false, guestResp.Body["can_see_exact_location"])
	require.Nil(t, guestResp.Body["address"])

	// Applicant подаёт заявку — pending, всё ещё не видит
	env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: applicantToken,
	})
	pendingResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: applicantToken,
	})
	require.Equal(t, false, pendingResp.Body["can_see_exact_location"])

	// Одобряем
	listResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID + "/applications", token: organizerToken,
	})
	apps := listResp.Body["applications"].([]any)
	appID := apps[0].(map[string]any)["id"].(string)

	env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID + "/approve", token: organizerToken,
	})
	approvedResp := env.srv.do(t, httpReq{
		method: "GET", path: "/api/v1/events/" + eventID, token: applicantToken,
	})
	require.Equal(t, true, approvedResp.Body["can_see_exact_location"])
	require.NotNil(t, approvedResp.Body["address"])

	// Гость: lat/lng присутствуют, но маскированы (не совпадают с точными).
	latGuest, _ := guestResp.Body["lat"].(float64)
	lngGuest, _ := guestResp.Body["lng"].(float64)
	require.NotZero(t, latGuest, "guest must receive masked lat")
	require.NotZero(t, lngGuest, "guest must receive masked lng")
	require.NotEqual(t, 51.53, latGuest, "guest lat must be masked")
	require.NotEqual(t, 46.03, lngGuest, "guest lng must be masked")
	require.Nil(t, guestResp.Body["address"])

	// Approved: точные координаты.
	latAppr, _ := approvedResp.Body["lat"].(float64)
	lngAppr, _ := approvedResp.Body["lng"].(float64)
	require.InDelta(t, 51.53, latAppr, 1e-6, "approved must see exact lat")
	require.InDelta(t, 46.03, lngAppr, 1e-6, "approved must see exact lng")
}

func TestEventLimits(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	organizerEmail := uniqueEmail("org_limits")
	organizerToken := env.registerAndLogin(t, organizerEmail, "ol_"+shortID())

	// Создаём 2 события — ок
	for i := 0; i < 2; i++ {
		resp := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events", token: organizerToken,
			body: map[string]any{
				"title": "Event", "category": "прогулки",
				"starts_at": futureTime(24*time.Hour + time.Duration(i)*time.Hour).Format("2006-01-02T15:04:05Z07:00"),
				"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			},
		})
		require.Equal(t, 201, resp.Status, resp.Raw)
	}

	// Третье — 400
	thirdResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: organizerToken,
		body: map[string]any{
			"title": "Event 3", "category": "спорт",
			"starts_at": futureTime(72 * time.Hour).Format("2006-01-02T15:04:05Z07:00"),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 400, thirdResp.Status)
	require.Contains(t, thirdResp.Body["message"], "Больше двух активных событий не создать")
}

func TestApplicationLimit(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	organizerToken := env.registerAndLogin(t, uniqueEmail("org_applimit"), "oa_"+shortID())
	userToken := env.registerAndLogin(t, uniqueEmail("user_applimit"), "ua_"+shortID())
	organizer2Token := env.registerAndLogin(t, uniqueEmail("org2_applimit"), "o2_"+shortID())

	eventIDs := []string{}
	for i := 0; i < 2; i++ {
		resp := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events", token: organizerToken,
			body: map[string]any{
				"title": "E1", "category": "прогулки",
				"starts_at": futureTime(24*time.Hour + time.Duration(i)*time.Hour).Format("2006-01-02T15:04:05Z07:00"),
				"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			},
		})
		require.Equal(t, 201, resp.Status, resp.Raw)
		eventIDs = append(eventIDs, resp.Body["id"].(string))
	}
	for i := 0; i < 2; i++ {
		resp := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events", token: organizer2Token,
			body: map[string]any{
				"title": "E2", "category": "спорт",
				"starts_at": futureTime(24*time.Hour + time.Duration(i)*time.Hour).Format("2006-01-02T15:04:05Z07:00"),
				"city":      "Саратов", "lat": 51.53, "lng": 46.03,
			},
		})
		require.Equal(t, 201, resp.Status, resp.Raw)
		eventIDs = append(eventIDs, resp.Body["id"].(string))
	}

	for i := 0; i < 3; i++ {
		resp := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events/" + eventIDs[i] + "/applications", token: userToken,
		})
		require.Equal(t, 201, resp.Status, resp.Raw)
	}

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventIDs[3] + "/applications", token: userToken,
	})
	require.Equal(t, 400, resp.Status)
	require.Contains(t, resp.Body["message"], "Больше трех заявок")
}

func TestHiddenLocationFeed(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("org_hlf"), "ohlf_"+shortID())
	approvedToken := env.registerAndLogin(t, uniqueEmail("appr_hlf"), "aprh_"+shortID())

	// Уникальный city — чтобы не зависеть от событий прошлых тестов и пагинации.
	// ОДИН на весь тест, не внутри findInFeed.
	uniqueCity := "Саратов-" + shortID()

	eventResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Hidden feed", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      uniqueCity, "address": "Точный адрес",
			"lat": 51.5300, "lng": 46.0300,
			"hide_exact_location": true,
		},
	})
	require.Equal(t, 201, eventResp.Status, eventResp.Raw)
	eventID := eventResp.Body["id"].(string)

	applyResp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: approvedToken,
	})
	require.Equal(t, 201, applyResp.Status, applyResp.Raw)
	appID := applyResp.Body["id"].(string)
	approveResp := env.srv.do(t, httpReq{
		method: "PATCH", path: "/api/v1/applications/" + appID + "/approve", token: orgToken,
	})
	require.Equal(t, 200, approveResp.Status, approveResp.Raw)

	findInFeed := func(token string) map[string]any {
		resp := env.srv.do(t, httpReq{
			method: "GET",
			path:   "/api/v1/events?city=" + url.QueryEscape(uniqueCity),
			token:  token,
		})
		require.Equal(t, 200, resp.Status, resp.Raw)
		for _, e := range resp.Body["events"].([]any) {
			m := e.(map[string]any)
			if m["id"] == eventID {
				return m
			}
		}
		t.Fatalf("event %s not found in feed (city=%s)", eventID, uniqueCity)
		return nil
	}

	guest := findInFeed("")
	require.Equal(t, false, guest["can_see_exact_location"])
	require.Nil(t, guest["address"])
	latGuest, _ := guest["lat"].(float64)
	lngGuest, _ := guest["lng"].(float64)
	require.NotZero(t, latGuest)
	require.NotZero(t, lngGuest)
	require.NotEqual(t, 51.5300, latGuest)
	require.NotEqual(t, 46.0300, lngGuest)

	appr := findInFeed(approvedToken)
	require.Equal(t, true, appr["can_see_exact_location"])
	require.Equal(t, "Точный адрес", appr["address"])
	latAppr, _ := appr["lat"].(float64)
	lngAppr, _ := appr["lng"].(float64)
	require.InDelta(t, 51.5300, latAppr, 1e-6)
	require.InDelta(t, 46.0300, lngAppr, 1e-6)

	org := findInFeed(orgToken)
	require.Equal(t, true, org["can_see_exact_location"])
	latOrg, _ := org["lat"].(float64)
	lngOrg, _ := org["lng"].(float64)
	require.InDelta(t, 51.5300, latOrg, 1e-6)
	require.InDelta(t, 46.0300, lngOrg, 1e-6)
}
