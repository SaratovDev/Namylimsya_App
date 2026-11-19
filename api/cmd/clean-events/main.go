package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	// Защита от случайного запуска в проде: DELETE FROM events без
	// подтверждения = потеря данных. Требуем явный флаг.
	if len(os.Args) < 2 || os.Args[1] != "--yes-i-really-mean-it" {
		fmt.Fprintln(os.Stderr, "clean-events: DELETE FROM events. This is DESTRUCTIVE.")
		fmt.Fprintln(os.Stderr, "Usage: clean-events --yes-i-really-mean-it")
		os.Exit(1)
	}

	// .env ищем как config.Load — в текущей, затем в родительских
	for _, p := range []string{".env", "../.env", "../../.env", "../../../.env"} {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			break
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer pool.Close()

	tag, err := pool.Exec(ctx, "DELETE FROM events")
	if err != nil {
		fmt.Fprintln(os.Stderr, "delete:", err)
		os.Exit(1)
	}
	fmt.Printf("deleted %d events\n", tag.RowsAffected())
}
