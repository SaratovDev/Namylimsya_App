package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
)

var rlTestCounter uint64

// TestAuthRateLimit проверяет, что /auth/login блокируется после N запросов с одного IP.
func TestAuthRateLimit(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	auth.SetCost(4)
	cfg, err := config.Load()
	require.NoError(t, err)
	ctx := context.Background()

	rdb, err := cache.NewRedis(ctx, cfg.RedisURL)
	require.NoError(t, err)

	pgPool, err := db.NewPostgres(ctx, cfg.DatabaseURL)
	require.NoError(t, err)
	defer pgPool.Close()

	tm := auth.NewTokenManager(cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	sm := auth.NewSessionManager(rdb)
	queries := dbgen.New(pgPool)

	localStorage, err := storage.NewLocalStorage(cfg.Storage.LocalPath, cfg.Storage.PublicBaseURL+"/static")
	require.NoError(t, err)

	mailer := email.NewSender("localhost", "1025", "", "", "noreply@test.local", true)

	authSvc := services.NewAuthService(queries, tm, sm, rdb)
	authHandler := handlers.NewAuthHandler(authSvc, cfg, mailer)
	userSvc := services.NewUserService(queries, localStorage, cfg.Storage.PublicBaseURL)
	userHandler := handlers.NewUserHandler(userSvc)
	hub := ws.NewHub(3)
	chatSvc := services.NewChatService(queries)
	chatHandler := handlers.NewChatHandler(chatSvc, hub)
	rlForWS := ratelimit.New(rdb)
	wsHandler := handlers.NewWSHandler(hub, chatSvc, tm, sm, rlForWS, "http://localhost:3000")
	eventSvc := services.NewEventService(queries, pgPool, nil)
	appSvc := services.NewApplicationService(queries, pgPool, nil)
	eventHandler := handlers.NewEventHandler(eventSvc, appSvc)
	appHandler := handlers.NewApplicationHandler(appSvc)
	adminSvc := services.NewAdminService(queries, pgPool, sm, nil, cfg.JWT.AccessTTL)
	adminHandler := handlers.NewAdminHandler(adminSvc)

	// Лимитер — с малым лимитом для теста.
	testRL := ratelimit.New(rdb)

	r := router.New(router.Deps{
		Health:       &handlers.HealthHandler{DB: pgPool, Redis: rdb},
		Auth:         authHandler,
		User:         userHandler,
		Event:        eventHandler,
		Application:  appHandler,
		Chat:         chatHandler,
		WS:           wsHandler,
		TM:           tm,
		SM:           sm,
		Admin:        adminHandler,
		RL:           testRL,
		StoragePath:  cfg.Storage.LocalPath,
		TrustedProxy: false,
	})

	// Уникальный IP: глобальный счётчик + наносекунды, чтобы исключить
	// коллизии с прошлыми прогонами в Redis. `%250` давал 250 комбинаций —
	// при повторных прогонах ключ `rl:auth_ip:10.99.X.1` оставался
	// (TTL = 2*window = 2 минуты), и счётчик стартовал не с нуля.
	n := atomic.AddUint64(&rlTestCounter, 1)
	uniqueIP := fmt.Sprintf("10.99.%d.%d",
		byte(n>>8), byte(n))

	// Сброс возможного leftover от прошлых прогонов (на всякий случай).
	_ = rdb.Del(ctx, "rl:auth_ip:"+uniqueIP).Err()

	// Отправляем 10 запросов — все должны пройти (не 429).
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = uniqueIP + ":12345"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.NotEqual(t, http.StatusTooManyRequests, w.Code,
			"request %d must not be rate-limited, got %d", i, w.Code)
	}

	// 11-й — 429.
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req.RemoteAddr = uniqueIP + ":12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusTooManyRequests, w.Code, "11th request must be 429")
	require.NotEmpty(t, w.Header().Get("Retry-After"))

	// Другой IP — не блокируется.
	req2 := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req2.RemoteAddr = "10.99.254.1:9999"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.NotEqual(t, http.StatusTooManyRequests, w2.Code, "different IP must not be blocked")

	_ = env // env нужен только для проверки, что INTEGRATION=1 и БД доступна
	_ = time.Second
}

// Тест на обход rate-limit через spoofed headers.
// Воспроизводит N1: раньше X-Real-IP / True-Client-IP читались первыми,
// и клиент мог подделать IP заголовком. Сейчас — игнорируются, XFF берётся
// правый (его дописал Caddy).
func TestAuthRateLimit_SpoofedHeadersBypass(t *testing.T) {
	// Тот же setup, что в TestAuthRateLimit, но с TrustedProxy=true
	env := newTestEnv(t)
	defer env.cleanup()

	cfg, err := config.Load()
	require.NoError(t, err)
	ctx := context.Background()

	rdb, err := cache.NewRedis(ctx, cfg.RedisURL)
	require.NoError(t, err)

	pgPool, err := db.NewPostgres(ctx, cfg.DatabaseURL)
	require.NoError(t, err)
	defer pgPool.Close()

	tm := auth.NewTokenManager(cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	sm := auth.NewSessionManager(rdb)
	queries := dbgen.New(pgPool)

	mailer := email.NewSender("localhost", "1025", "", "", "noreply@test.local", true)

	localStorage, err := storage.NewLocalStorage(cfg.Storage.LocalPath, "http://localhost:"+cfg.HTTPPort+"/static")
	require.NoError(t, err)
	authSvc := services.NewAuthService(queries, tm, sm, rdb)
	authHandler := handlers.NewAuthHandler(authSvc, cfg, mailer)
	userSvc := services.NewUserService(queries, localStorage, cfg.Storage.PublicBaseURL)
	userHandler := handlers.NewUserHandler(userSvc)
	hub := ws.NewHub(3)
	chatSvc := services.NewChatService(queries)
	chatHandler := handlers.NewChatHandler(chatSvc, hub)
	rlForWS := ratelimit.New(rdb)
	wsHandler := handlers.NewWSHandler(hub, chatSvc, tm, sm, rlForWS, "http://localhost:3000")
	eventSvc := services.NewEventService(queries, pgPool, nil)
	appSvc := services.NewApplicationService(queries, pgPool, nil)
	eventHandler := handlers.NewEventHandler(eventSvc, appSvc)
	appHandler := handlers.NewApplicationHandler(appSvc)
	adminSvc := services.NewAdminService(queries, pgPool, sm, nil, cfg.JWT.AccessTTL)
	adminHandler := handlers.NewAdminHandler(adminSvc)

	testRL := ratelimit.New(rdb)

	r := router.New(router.Deps{
		Health:       &handlers.HealthHandler{DB: pgPool, Redis: rdb},
		Auth:         authHandler,
		User:         userHandler,
		Event:        eventHandler,
		Application:  appHandler,
		Chat:         chatHandler,
		WS:           wsHandler,
		TM:           tm,
		SM:           sm,
		Admin:        adminHandler,
		RL:           testRL,
		StoragePath:  cfg.Storage.LocalPath,
		TrustedProxy: true, // ← prod-топология: доверяем XFF от Caddy
	})

	// Сброс leftover.
	realIP := "172.18.0.9" // "клиент", которого видит Caddy
	_ = rdb.Del(ctx, "rl:auth_ip:"+realIP).Err()

	// "Клиент" — private peer (Docker bridge от Caddy). На каждый запрос —
	// НОВЫЙ поддельный X-Real-IP и новый левый элемент XFF. Правый элемент
	// XFF постоянен — его в проде дописывает Caddy.
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = "172.17.0.2:12345"
		req.Header.Set("X-Real-IP", fmt.Sprintf("41.0.%d.%d", byte(i>>8), byte(i)))
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("42.0.%d.%d, %s", byte(i>>8), byte(i), realIP))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			return // лимит сработал — bypass закрыт
		}
	}
	t.Fatal("15 запросов с рандомными X-Real-IP/XFF не получили 429 — rate-limit обходится")
}
