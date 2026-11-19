package integration

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// pending_applications_count в публичной ленте не должен утекать:
// только организатор видит реальное число, все остальные — 0.
func TestFeedDoesNotLeakPendingCount(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("leak_org"), "lo_"+shortID())
	applicantToken := env.registerAndLogin(t, uniqueEmail("leak_app"), "la_"+shortID())
	anotherToken := env.registerAndLogin(t, uniqueEmail("leak_oth"), "loth_"+shortID())

	uniqueCity := "Саратов-" + shortID()

	resp := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Leak test", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      uniqueCity, "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, resp.Status, resp.Raw)
	eventID := resp.Body["id"].(string)

	// Две pending-заявки на событие.
	for _, tok := range []string{applicantToken, anotherToken} {
		r := env.srv.do(t, httpReq{
			method: "POST", path: "/api/v1/events/" + eventID + "/applications", token: tok,
		})
		require.Equal(t, 201, r.Status, r.Raw)
	}

	findInFeed := func(token string) map[string]any {
		r := env.srv.do(t, httpReq{
			method: "GET",
			path:   "/api/v1/events?city=" + url.QueryEscape(uniqueCity),
			token:  token,
		})
		require.Equal(t, 200, r.Status, r.Raw)
		for _, e := range r.Body["events"].([]any) {
			m := e.(map[string]any)
			if m["id"] == eventID {
				return m
			}
		}
		t.Fatalf("event %s not found in feed (city=%s)", eventID, uniqueCity)
		return nil
	}

	// Гость: 0, поле присутствует, но без реального числа.
	guest := findInFeed("")
	require.EqualValues(t, 0, guest["pending_applications_count"],
		"guest must not see pending count")

	// Pending-участник (не орг): 0.
	pend := findInFeed(applicantToken)
	require.EqualValues(t, 0, pend["pending_applications_count"],
		"pending applicant must not see pending count")

	// Второй pending-участник: 0.
	another := findInFeed(anotherToken)
	require.EqualValues(t, 0, another["pending_applications_count"],
		"other applicant must not see pending count")

	// Организатор: реальное число = 2.
	org := findInFeed(orgToken)
	require.EqualValues(t, 2, org["pending_applications_count"],
		"organizer must see real pending count")
}
