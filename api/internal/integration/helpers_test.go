package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/cache"
	"github.com/SaratovDev/Namylimsya_App/api/internal/config"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/email"
	"github.com/SaratovDev/Namylimsya_App/api/internal/handlers"
	"github.com/SaratovDev/Namylimsya_App/api/internal/ratelimit"
	"github.com/SaratovDev/Namylimsya_App/api/internal/router"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
	"github.com/SaratovDev/Namylimsya_App/api/internal/storage"
	"github.com/SaratovDev/Namylimsya_App/api/internal/ws"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

var shortIDCounter atomic.Uint64

// shortID — уникальный суффикс для имён тестовых сущностей (username, city).
// Раньше был %100000 от UnixNano — при параллельных прогонах и накоплении
// данных в БД это давало коллизии (два теста создают city «Саратов-74100»
// и мешают друг другу). Теперь монотонный счётчик + наносекунды.
func shortID() string {
	return fmt.Sprintf("%d_%d", time.Now().UnixNano(), shortIDCounter.Add(1))
}

func newRequest(method, url string, body *bytes.Buffer) (*http.Request, error) {
	return http.NewRequest(method, url, body)
}

func newRecorder() *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}

// cleanup закрывает пул Postgres и Redis-клиент, созданные в newTestEnv.
// Без этого соединения накапливаются между тестами и упираются в
// max_connections Postgres.
func (e *testEnv) cleanup() {
	if e.pgPool != nil {
		e.pgPool.Close()
		e.pgPool = nil
	}
	if e.rdb != nil {
		_ = e.rdb.Close()
		e.rdb = nil
	}
}

// testEnv — общее окружение для всех тестов в пакете
type testEnv struct {
	srv     *testServer
	pgPool  *pgxpool.Pool
	queries *dbgen.Queries
	rdb     *redis.Client
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

	// bcrypt cost=4 вместо прод-14 — иначе интеграционные тесты упираются
	// в общий таймаут go test (10m) из-за 100+ медленных хэшей.
	auth.SetCost(4)

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
		cfg.Storage.LocalPath, cfg.Storage.PublicBaseURL+"/static",
	)
	require.NoError(t, err)

	mailer := email.NewSender("localhost", "1025", "", "", "noreply@test.local", true)

	// Инициализация сервисов и хендлеров — как в main.go
	health := &handlers.HealthHandler{DB: pgPool, Redis: rdb}
	authSvc := services.NewAuthService(queries, tm, sm, rdb)
	authHandler := handlers.NewAuthHandler(authSvc, cfg, mailer)
	hub := ws.NewHub(3)
	chatSvc := services.NewChatService(queries)
	chatHandler := handlers.NewChatHandler(chatSvc, hub)
	rl := ratelimit.New(rdb)
	wsHandler := handlers.NewWSHandler(hub, chatSvc, tm, sm, rl, "http://localhost:3000")
	userSvc := services.NewUserService(queries, localStorage, cfg.Storage.PublicBaseURL)
	userHandler := handlers.NewUserHandler(userSvc)

	eventSvc := services.NewEventService(queries, pgPool, nil)
	appSvc := services.NewApplicationService(queries, pgPool, nil)
	eventHandler := handlers.NewEventHandler(eventSvc, appSvc)
	appHandler := handlers.NewApplicationHandler(appSvc)

	// Admin (зависит от sm; notif nil — уведомления в тестах не критичны)
	adminSvc := services.NewAdminService(queries, pgPool, sm, nil, cfg.JWT.AccessTTL)
	adminHandler := handlers.NewAdminHandler(adminSvc)

	// Support (зависит от adminSvc для audit log)
	supportSvc := services.NewSupportService(queries, adminSvc, nil)
	supportHandler := handlers.NewSupportHandler(supportSvc)

	r := router.New(router.Deps{
		Health:       health,
		Auth:         authHandler,
		User:         userHandler,
		Event:        eventHandler,
		Application:  appHandler,
		Chat:         chatHandler,
		WS:           wsHandler,
		TM:           tm,
		SM:           sm,
		Admin:        adminHandler,
		Support:      supportHandler,
		StoragePath:  cfg.Storage.LocalPath,
		RL:           rl,
		TrustedProxy: false, // в тестах нет proxy, XFF не нужен
	})

	return &testEnv{
		srv:     &testServer{Handler: r, tm: tm},
		pgPool:  pgPool,
		queries: queries,
		rdb:     rdb,
	}
}

// httpReq — удобный хелпер для запросов
type httpReq struct {
	method  string
	path    string
	token   string
	body    any
	cookies string
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
	req.RemoteAddr = uniqueRemoteAddr()
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

	// Тесты передают «префикс_shortID», но shortID коллизионен (всего 100k
	// значений). Добавляем UUID-хвост, чтобы username гарантированно был
	// уникальным и регистрация не падала с 409 по username.
	username = username + "_" + uuid.NewString()[:8]

	resp := e.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/register",
		body: map[string]any{
			"email": email, "password": "secret123", "username": username, "city": "Саратов",
			"consent_accepted": true,
			"age_confirmed":    true,
			"consent_version":  "1.0",
		},
	})
	// 409 допустим ТОЛЬКО когда пользователь с таким email уже есть (повторный
	// прогон). Если 409, но юзера по email нет — это конфликт username.
	if resp.Status == 409 {
		_, err := e.queries.GetUserByEmail(context.Background(), email)
		require.NoError(t, err, "register conflict, but user by email not found (username collision?)")
	} else {
		require.Equal(t, 201, resp.Status, "register %s: %s", email, resp.Raw)
	}

	resp = e.srv.do(t, httpReq{
		method: "POST", path: "/api/v1/auth/login",
		body: map[string]any{"email": email, "password": "secret123"},
	})
	require.Equal(t, 200, resp.Status, "login %s: %s", email, resp.Raw)
	token, _ := resp.Body["access_token"].(string)
	require.NotEmpty(t, token)
	return token
}

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("itest+%s+%s@example.com", prefix, uuid.NewString())
}

// futureTime — защита от «событие в прошлом» (starts_at должен быть в будущем).
func futureTime(d time.Duration) time.Time { return time.Now().Add(d) }

// doRaw выполняет запрос и возвращает полный *http.Response (для проверки Set-Cookie).
func (s *testServer) doRaw(t *testing.T, r httpReq) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if r.body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(r.body))
	}
	req, err := newRequest(r.method, r.path, &buf)
	require.NoError(t, err)
	req.RemoteAddr = uniqueRemoteAddr()
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	if r.cookies != "" {
		req.Header.Set("Cookie", r.cookies)
	}
	req.Header.Set("Content-Type", "application/json")

	w := newRecorder()
	s.ServeHTTP(w, req)
	return w.Result()
}

// loginRaw возвращает access_token и Set-Cookie из ответа /auth/login.
func (e *testEnv) loginRaw(t *testing.T, email, password string) (accessToken string, setCookies []string) {
	t.Helper()
	resp := e.srv.doRaw(t, httpReq{
		method: "POST", path: "/api/v1/auth/login",
		body: map[string]any{"email": email, "password": password},
	})
	require.Equal(t, 200, resp.StatusCode, "login status")
	defer resp.Body.Close()

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	accessToken, _ = body["access_token"].(string)
	require.NotEmpty(t, accessToken, "access_token must be present")

	setCookies = resp.Header.Values("Set-Cookie")
	return accessToken, setCookies
}

// findCookie возвращает значение Set-Cookie header'а, содержащего name=.
func findCookie(headers []string, name string) string {
	for _, h := range headers {
		if strings.HasPrefix(h, name+"=") {
			return h
		}
	}
	return ""
}

// cookieKV извлекает пару "name=value" из строки Set-Cookie (без атрибутов).
func cookieKV(setCookieHeader string) string {
	if i := strings.Index(setCookieHeader, ";"); i >= 0 {
		return setCookieHeader[:i]
	}
	return setCookieHeader
}

var remoteAddrCounter atomic.Uint64

// uniqueRemoteAddr — уникальный IP на каждый тестовый запрос, чтобы
// IP rate limit на /auth/login (10/мин с одного IP) не срабатывал при
// прогоне всего пакета. В prod IP приходит от reverse-proxy.
func uniqueRemoteAddr() string {
	n := remoteAddrCounter.Add(1)
	return fmt.Sprintf("10.%d.%d.%d:12345",
		byte(n>>16), byte(n>>8), byte(n))
}
