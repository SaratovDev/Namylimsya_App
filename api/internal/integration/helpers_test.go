package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/cache"
	"github.com/SaratovDev/Namylimsya_App/api/internal/config"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/handlers"
	"github.com/SaratovDev/Namylimsya_App/api/internal/router"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
	"github.com/SaratovDev/Namylimsya_App/api/internal/storage"
	"github.com/stretchr/testify/require"
)

func shortID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%100000)
}

func newRequest(method, url string, body *bytes.Buffer) (*http.Request, error) {
	return http.NewRequest(method, url, body)
}

func newRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

// cleanup — заглушка (используем уникальные email'ы)
func (e *testEnv) cleanup() {}

// testEnv — общее окружение для всех тестов в пакете
type testEnv struct {
	srv     *testServer
	pgPool  interface{ Close() }
	queries *dbgen.Queries
}

type testServer struct {
	http.Handler
	tm *auth.TokenManager
}

// newTestEnv поднимает всё: БД, Redis, роутер. Пропускает тест, если INTEGRATION != 1.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("set INTEGRATION=1 to run integration tests")
	}

	cfg, err := config.Load()
	require.NoError(t, err, "load config")

	ctx := context.Background()

	pgPool, err := db.NewPostgres(ctx, cfg.DatabaseURL)
	require.NoError(t, err, "connect postgres")

	rdb, err := cache.NewRedis(ctx, cfg.RedisURL)
	require.NoError(t, err, "connect redis")

	tm := auth.NewTokenManager(
		cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret,
		cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL,
	)
	sm := auth.NewSessionManager(rdb)

	queries := dbgen.New(pgPool)

	localStorage, err := storage.NewLocalStorage(
		cfg.Storage.LocalPath, "http://localhost:"+cfg.HTTPPort+"/static",
	)
	require.NoError(t, err)

	// Инициализация сервисов и хендлеров — как в main.go
	health := &handlers.HealthHandler{DB: pgPool, Redis: rdb}
	authSvc := services.NewAuthService(queries, tm, sm)
	authHandler := handlers.NewAuthHandler(authSvc)

	userSvc := services.NewUserService(queries, localStorage)
	userHandler := handlers.NewUserHandler(userSvc)

	eventSvc := services.NewEventService(queries)
	appSvc := services.NewApplicationService(queries)
	eventHandler := handlers.NewEventHandler(eventSvc, appSvc)
	appHandler := handlers.NewApplicationHandler(appSvc)

	r := router.New(router.Deps{
		Health:      health,
		Auth:        authHandler,
		User:        userHandler,
		Event:       eventHandler,
		Application: appHandler,
		TM:          tm,
	})

	return &testEnv{
		srv:     &testServer{Handler: r, tm: tm},
		pgPool:  pgPool,
		queries: queries,
	}
}

// httpReq — удобный хелпер для запросов
type httpReq struct {
	method string
	path   string
	token  string
	body   any
}

type httpResp struct {
	Status int
	Body   map[string]any
	Raw    string
}

func (s *testServer) do(t *testing.T, r httpReq) httpResp {
	t.Helper()
	var buf bytes.Buffer
	if r.body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(r.body))
	}
	req, err := newRequest(r.method, r.path, &buf)
	require.NoError(t, err)
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	req.Header.Set("Content-Type", "application/json")

	w := newRecorder()
	s.ServeHTTP(w, req)

	resp := httpResp{Status: w.Code, Raw: w.Body.String()}
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &resp.Body)
	}
	return resp
}

// registerAndLogin регистрирует пользователя и возвращает access token.
func (e *testEnv) registerAndLogin(t *testing.T, email, username string) string {
	t.Helper()
	// Регистрация
	resp := e.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/register",
		body: map[string]any{
			"email": email, "password": "secret123", "username": username, "city": "Саратов",
		},
	})
	// Если уже существует (повторный запуск) — ок
	if resp.Status != 201 && resp.Status != 409 {
		t.Fatalf("register %s: status %d, body %s", email, resp.Status, resp.Raw)
	}

	// Логин
	resp = e.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/login",
		body: map[string]any{"email": email, "password": "secret123"},
	})
	require.Equal(t, 200, resp.Status, "login %s: %s", email, resp.Raw)
	token, _ := resp.Body["access_token"].(string)
	require.NotEmpty(t, token)
	return token
}

// cleanupUser удаляет пользователя по email (каскадно удалит events, applications и т.д.).
func uniqueEmail(prefix string) string {
	return fmt.Sprintf("itest+%s+%d@example.com", prefix, time.Now().UnixNano())
}

// ttlGuard — защита от «событие в прошлом» (starts_at должен быть в будущем).
func futureTime(d time.Duration) time.Time { return time.Now().Add(d) }
