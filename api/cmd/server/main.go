package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

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

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}

	// Уровень логирования из LOG_LEVEL (debug|info|warn|error). В проде по
	// умолчанию info — чтобы не светить debug-строки и SQL-трейсеры.
	if lvl, err := zerolog.ParseLevel(cfg.LogLevel); err == nil {
		zerolog.SetGlobalLevel(lvl)
	}

	ctx, cancel := context.WithCancel(context.Background())

	pgPool, err := db.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect postgres")
	}
	defer pgPool.Close()
	log.Info().Msg("postgres connected")

	rdb, err := cache.NewRedis(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect redis")
	}
	defer rdb.Close()
	log.Info().Msg("redis connected")

	// --- Email ---
	mailer := email.NewSender(
		cfg.SMTP.Host,
		cfg.SMTP.Port,
		cfg.SMTP.User,
		cfg.SMTP.Pass,
		cfg.SMTP.From,
		!cfg.SMTP.Enabled, // независимый флаг SMTP_ENABLED
	)

	// --- Auth ---
	tm := auth.NewTokenManager(
		cfg.JWT.AccessSecret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessTTL,
		cfg.JWT.RefreshTTL,
	)
	sm := auth.NewSessionManager(rdb)

	queries := dbgen.New(pgPool)

	authSvc := services.NewAuthService(queries, tm, sm, rdb)
	authHandler := handlers.NewAuthHandler(authSvc, cfg, mailer)

	// --- Storage ---
	localStorage, err := storage.NewLocalStorage(
		cfg.Storage.LocalPath,
		cfg.Storage.PublicBaseURL+"/static",
	)
	if err != nil {
		log.Fatal().Err(err).Msg("init storage")
	}

	// --- Notifications ---
	notifSvc := services.NewNotificationService(queries, mailer)
	notifWorker := services.NewNotificationWorker(queries, mailer, cfg.PublicBaseURL)
	notifSvc.SetEmailEnqueuer(notifWorker.Enqueue)

	// Запуск воркера email-батчинга в фоне.
	// Start() сам делает wg.Add(1) и запускает горутину — внешний `go` не нужен
	// и создаёт гонку между Add и Wait на shutdown.
	notifWorker.Start(ctx)

	// Автозавершение прошедших событий
	eventWorker := services.NewEventWorker(queries)
	eventWorker.Start(ctx)

	defer func() {
		cancel()           // сначала триггерим ctx.Done, чтобы воркеры вышли из цикла
		notifWorker.Stop() // затем ждём завершения — wg.Wait() разблокируется
		eventWorker.Stop()
	}()

	notifHub := ws.NewNotificationHub(3)
	notifSvc.SetWSNotifier(func(userID uuid.UUID, payload map[string]any) {
		notifHub.SendToUser(userID, map[string]any{
			"type": "notification",
			"data": payload,
		})
	})

	// --- Admin (зависит от notifSvc) ---
	adminSvc := services.NewAdminService(queries, pgPool, sm, notifSvc, cfg.JWT.AccessTTL)
	adminHandler := handlers.NewAdminHandler(adminSvc)
	notifHandler := handlers.NewNotificationHandler(notifSvc)
	wsNotifHandler := handlers.NewWSNotificationHandler(notifHub, tm, sm, cfg.PublicBaseURL)

	// --- Chat ---
	hub := ws.NewHub(3)
	chatSvc := services.NewChatService(queries)
	chatHandler := handlers.NewChatHandler(chatSvc, hub)
	rl := ratelimit.New(rdb)
	wsHandler := handlers.NewWSHandler(hub, chatSvc, tm, sm, rl, cfg.PublicBaseURL)
	// --- User ---
	userSvc := services.NewUserService(queries, localStorage, cfg.Storage.PublicBaseURL)
	userHandler := handlers.NewUserHandler(userSvc)

	// --- Applications (с notifSvc) ---
	appSvc := services.NewApplicationService(queries, pgPool, notifSvc)
	appHandler := handlers.NewApplicationHandler(appSvc)

	supportSvc := services.NewSupportService(queries, adminSvc, mailer)
	supportHandler := handlers.NewSupportHandler(supportSvc)

	// --- Events (с notifSvc) ---
	eventSvc := services.NewEventService(queries, pgPool, notifSvc)
	eventHandler := handlers.NewEventHandler(eventSvc, appSvc)

	// --- Router ---
	health := &handlers.HealthHandler{DB: pgPool, Redis: rdb}
	var rlPtr *ratelimit.Limiter
	if !cfg.DisableRateLimit {
		rlPtr = rl
	}
	r := router.New(router.Deps{
		Health:         health,
		Auth:           authHandler,
		User:           userHandler,
		Event:          eventHandler,
		Application:    appHandler,
		Chat:           chatHandler,
		WS:             wsHandler,
		Notification:   notifHandler,
		WSNotification: wsNotifHandler,
		TM:             tm,
		SM:             sm,
		Admin:          adminHandler,
		Support:        supportHandler,
		CORSOrigins:    []string{cfg.PublicBaseURL},
		RL:             rlPtr,
		StoragePath:    cfg.Storage.LocalPath,
		TrustedProxy:   cfg.TrustedProxy,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// WriteTimeout намеренно не задаём (0 = без лимита):
		// он глобальный для всех хендлеров, включая WebSocket,
		// и рвёт долгоживущие WS-соединения. От медленного чтения ответа
		// защищает reverse-proxy в проде.
		IdleTimeout: 60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("http server listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("listen")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("shutdown")
	}
	log.Info().Msg("server stopped")
}
