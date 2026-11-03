package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/cache"
	"github.com/SaratovDev/Namylimsya_App/api/internal/config"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/handlers"
	"github.com/SaratovDev/Namylimsya_App/api/internal/router"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	// --- Инициализация Auth ---
	tm := auth.NewTokenManager(
		cfg.JWT.AccessSecret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessTTL,
		cfg.JWT.RefreshTTL,
	)
	sm := auth.NewSessionManager(rdb)

	queries := dbgen.New(pgPool)

	authSvc := services.NewAuthService(queries, tm, sm)
	authHandler := handlers.NewAuthHandler(authSvc)

	// --- Роутер ---
	health := &handlers.HealthHandler{DB: pgPool, Redis: rdb}
	r := router.New(router.Deps{
		Health: health,
		Auth:   authHandler,
		TM:     tm,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
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
