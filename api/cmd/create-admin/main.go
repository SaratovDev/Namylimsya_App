// Команда create-admin создаёт (или повышает до admin) пользователя напрямую
// в БД. Нужна для первичного bootstrap — без неё первого администратора
// приходится назначать SQL-запросом вручную.
//
// Примеры:
//
//	# локально (из каталога api):
//	go run ./cmd/create-admin -email admin@example.com -username admin
//
//	# в Docker (контейнер api уже запущен, env полный):
//	docker compose -f infra/docker-compose.prod.yml --env-file .env.production \
//	    exec api /app/create-admin -email admin@example.com -username admin
//
// Пароль читается из env ADMIN_PASSWORD; если не задан — генерируется
// случайный и выводится в stdout.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/legal"
)

func main() {
	email := flag.String("email", "", "email администратора (обязательно)")
	username := flag.String("username", "", "username администратора (обязательно)")
	flag.Parse()

	*email = strings.ToLower(strings.TrimSpace(*email))
	*username = strings.TrimSpace(*username)
	if *email == "" || *username == "" {
		fmt.Fprintln(os.Stderr, "usage: create-admin -email <email> -username <username>")
		os.Exit(2)
	}

	password := os.Getenv("ADMIN_PASSWORD")
	if password == "" {
		password = randomPassword()
		fmt.Printf("ADMIN_PASSWORD не задан — сгенерирован: %s\n", password)
	}
	if len(password) < 8 {
		fmt.Fprintln(os.Stderr, "пароль должен быть не короче 8 символов")
		os.Exit(2)
	}

	dsn := loadDSN()
	if dsn == "" {
		log.Fatal().Msg("DATABASE_URL is required (создайте .env в корне репо или передайте env)")
	}

	ctx := context.Background()
	pool, err := db.NewPostgres(ctx, dsn)
	if err != nil {
		log.Fatal().Err(err).Msg("connect postgres")
	}
	defer pool.Close()

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatal().Err(err).Msg("hash password")
	}

	q := dbgen.New(pool)

	existing, err := q.GetUserByEmail(ctx, *email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Fatal().Err(err).Msg("get user by email")
	}

	var userID = existing.ID
	if errors.Is(err, pgx.ErrNoRows) {
		now := time.Now()
		consentVersion := legal.ConsentVersion
		created, err := q.CreateUser(ctx, dbgen.CreateUserParams{
			Email:          *email,
			PasswordHash:   hash,
			Username:       *username,
			ConsentAt:      &now,
			ConsentVersion: &consentVersion,
			AgeConfirmedAt: &now,
		})
		if err != nil {
			log.Fatal().Err(err).Msg("create user")
		}
		userID = created.ID
		fmt.Printf("создан пользователь %s (%s)\n", created.Username, created.Email)
	} else {
		if err := q.UpdateUserPassword(ctx, dbgen.UpdateUserPasswordParams{
			ID:           userID,
			PasswordHash: hash,
		}); err != nil {
			log.Fatal().Err(err).Msg("update password")
		}
		fmt.Printf("обновлён пароль существующего пользователя %s (%s)\n", existing.Username, existing.Email)
	}

	if _, err := q.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID:   userID,
		Role: "admin",
	}); err != nil {
		log.Fatal().Err(err).Msg("set admin role")
	}

	fmt.Printf("роль admin назначена (user_id=%s)\n", userID)
}

// loadDSN ищет .env в текущей и родительских директориях (как config.Load)
// и возвращает DATABASE_URL. Нужен только DSN — без APP_ENV/Redis, чтобы
// bootstrap-команда работала в минимальном окружении.
func loadDSN() string {
	for _, p := range []string{".env", "../.env", "../../.env", "../../../.env"} {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			break
		}
	}
	return os.Getenv("DATABASE_URL")
}

// randomPassword — криптостойкий случайный пароль (16 символов, base64url).
func randomPassword() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		log.Fatal().Err(err).Msg("crypto/rand")
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
