package db

import (
	"context"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/rs/zerolog/log"
)

func NewPostgres(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour

	// SQL-трейсер только вне integration-тестов и вне production: в тестах он даёт
	// ~90% вывода и мешает читать падения, а в проде пишет SQL с аргументами
	// (email и т.п.) в stdout — это PII и лишний объём логов.
	if os.Getenv("INTEGRATION") != "1" && os.Getenv("APP_ENV") != "production" {
		cfg.ConnConfig.Tracer = &tracelog.TraceLog{
			Logger: tracelog.LoggerFunc(func(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
				if level >= tracelog.LogLevelInfo {
					log.Debug().Msgf("pgx [%s]: %s | %v", level, msg, data)
				}
			}),
			LogLevel: tracelog.LogLevelDebug,
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
