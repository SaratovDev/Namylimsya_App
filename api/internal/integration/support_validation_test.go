package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Анонимный POST /support/tickets — 5 запросов проходят, 6-й блокируется.
func TestSupportTicketRateLimit(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Уникальный IP, чтобы не зацепить другие тесты и не наткнуться
	// на старые ключи в Redis от прошлых прогонов.
	uniqueIP := fmt.Sprintf("10.%d.%d.%d",
		byte(time.Now().UnixNano()>>16),
		byte(time.Now().UnixNano()>>8),
		byte(time.Now().UnixNano()),
	)

	body := `{"type":"suggestion","subject":"t","message":"m"}`

	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/v1/support/tickets", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = uniqueIP + ":12345"
		w := httptest.NewRecorder()
		env.srv.ServeHTTP(w, req)
		return w
	}

	for i := 0; i < 5; i++ {
		w := send()
		require.NotEqual(t, http.StatusTooManyRequests, w.Code,
			"request %d must pass (got %d: %s)", i, w.Code, w.Body.String())
	}

	w := send()
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.NotEmpty(t, w.Header().Get("Retry-After"))

	// Другой IP — не блокируется.
	otherIP := uniqueIP + ".99"
	req := httptest.NewRequest("POST", "/api/v1/support/tickets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = otherIP + ":12345"
	w2 := httptest.NewRecorder()
	env.srv.ServeHTTP(w2, req)
	require.NotEqual(t, http.StatusTooManyRequests, w2.Code)
}

// Валидация длины subject/message/reason.
func TestSupportInputLimits(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	token := env.registerAndLogin(t, uniqueEmail("lim_u"), "lim_"+shortID())

	// Длинный subject (301 символ).
	longSubject := strings.Repeat("a", 301)
	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/support/tickets", token: token,
		body: map[string]any{
			"type": "suggestion", "subject": longSubject, "message": "ok",
		},
	})
	require.Equal(t, http.StatusBadRequest, r.Status, r.Raw)

	// Длинное message (5001 символ).
	longMsg := strings.Repeat("a", 5001)
	r = env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/support/tickets", token: token,
		body: map[string]any{
			"type": "suggestion", "subject": "ok", "message": longMsg,
		},
	})
	require.Equal(t, http.StatusBadRequest, r.Status, r.Raw)

	// Пустое message.
	r = env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/support/tickets", token: token,
		body: map[string]any{
			"type": "suggestion", "subject": "ok", "message": "   ",
		},
	})
	require.Equal(t, http.StatusBadRequest, r.Status, r.Raw)

	// Валидный — 201.
	r = env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/support/tickets", token: token,
		body: map[string]any{
			"type": "suggestion", "subject": "ok", "message": "ok",
		},
	})
	require.Equal(t, http.StatusCreated, r.Status, r.Raw)
}

// Валидация reason у жалобы.
func TestComplaintReasonLimit(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	orgToken := env.registerAndLogin(t, uniqueEmail("crl_o"), "crlo_"+shortID())
	repToken := env.registerAndLogin(t, uniqueEmail("crl_r"), "crlr_"+shortID())

	r := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/events", token: orgToken,
		body: map[string]any{
			"title": "Target", "category": "прогулки",
			"starts_at": futureTime(24 * time.Hour).Format(time.RFC3339),
			"city":      "Саратов", "lat": 51.53, "lng": 46.03,
		},
	})
	require.Equal(t, 201, r.Status, r.Raw)
	eventID := r.Body["id"].(string)

	// Пустой reason.
	empty := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/complaints", token: repToken,
		body: map[string]any{
			"target_type": "event", "target_id": eventID, "reason": "   ",
		},
	})
	require.Equal(t, http.StatusBadRequest, empty.Status, empty.Raw)

	// Слишком длинный reason (1001 символ).
	long := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/complaints", token: repToken,
		body: map[string]any{
			"target_type": "event", "target_id": eventID,
			"reason": strings.Repeat("a", 1001),
		},
	})
	require.Equal(t, http.StatusBadRequest, long.Status, long.Raw)

	// Валидный — 201.
	ok := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/complaints", token: repToken,
		body: map[string]any{
			"target_type": "event", "target_id": eventID, "reason": "спам",
		},
	})
	require.Equal(t, http.StatusCreated, ok.Status, ok.Raw)
}

// Валидация длины ответа на тикет.
func TestTicketReplyBodyLimit(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	adminToken, _ := setupAdminComplaints(t, env)
	userToken := env.registerAndLogin(t, uniqueEmail("rbl_u"), "rbl_"+shortID())

	ticketID := createTicket(t, env, userToken, "suggestion", "Тема", "Тело")

	// Пустой body.
	empty := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/support/tickets/" + ticketID + "/reply",
		token: adminToken,
		body:  map[string]any{"body": "   "},
	})
	require.Equal(t, http.StatusBadRequest, empty.Status, empty.Raw)

	// Слишком длинный body (5001 символ).
	long := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/support/tickets/" + ticketID + "/reply",
		token: adminToken,
		body:  map[string]any{"body": strings.Repeat("a", 5001)},
	})
	require.Equal(t, http.StatusBadRequest, long.Status, long.Raw)

	// Валидный — 201.
	ok := env.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/admin/support/tickets/" + ticketID + "/reply",
		token: adminToken,
		body:  map[string]any{"body": "Ответ"},
	})
	require.Equal(t, http.StatusCreated, ok.Status, ok.Raw)
}
