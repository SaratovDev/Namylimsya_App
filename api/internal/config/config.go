package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv           string
	LogLevel         string
	HTTPPort         string
	PublicBaseURL    string
	DisableRateLimit bool // E2E_DISABLE_RATE_LIMIT — для Playwright

	DatabaseURL string
	RedisURL    string

	// TrustedProxy — true, когда API стоит за reverse-proxy (Caddy, nginx)
	// и X-Forwarded-For можно доверять (только от private peer'ов — Docker
	// bridge). false (default) — API напрямую, XFF игнорируется.
	TrustedProxy bool

	JWT struct {
		AccessSecret  string
		RefreshSecret string
		AccessTTL     time.Duration
		RefreshTTL    time.Duration
	}

	SMTP struct {
		Host    string
		Port    string
		User    string
		Pass    string
		From    string
		Enabled bool
	}

	Storage struct {
		Driver        string
		LocalPath     string
		PublicBaseURL string
	}

	YandexMapsAPIKey string
}

func Load() (*Config, error) {
	// .env в корне проекта; ищем в текущей, затем в родительских папках.
	// ВАЖНО: CWD у `go test ./internal/integration` — это сама директория пакета,
	// поэтому ../../../.env (корень репо) тоже нужен в списке.
	// Останавливаемся на первом найденном, чтобы непредсказуемо не смешивать файлы.
	for _, p := range []string{".env", "../.env", "../../.env", "../../../.env"} {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			break
		}
	}

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		// APP_ENV влияет на критичные вещи: валидацию JWT-секретов,
		// HTTPS-требование PUBLIC_BASE_URL, маскировку err.Error() в 500,
		// warn про SMTP. Без явного значения любая из этих проверок
		// молча отключается, и прод ведёт себя как dev. Поэтому
		// падаем с понятной ошибкой, а не подставляем дефолт.
		return nil, fmt.Errorf("APP_ENV is required (development | production)")
	}
	if appEnv != "development" && appEnv != "production" {
		return nil, fmt.Errorf("APP_ENV must be 'development' or 'production' (got %q)", appEnv)
	}

	cfg := &Config{
		AppEnv:        appEnv,
		LogLevel:      getEnv("LOG_LEVEL", defaultLogLevel(appEnv)),
		HTTPPort:      getEnv("HTTP_PORT", "8080"),
		PublicBaseURL: getEnv("PUBLIC_BASE_URL", "http://localhost:3000"), DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:         os.Getenv("REDIS_URL"),
		YandexMapsAPIKey: os.Getenv("YANDEX_MAPS_API_KEY"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required (create .env in repo root, or pass env explicitly)")
	}
	if cfg.RedisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is required (create .env in repo root, or pass env explicitly)")
	}

	cfg.TrustedProxy = getBool("TRUSTED_PROXY", false)
	cfg.DisableRateLimit = getBool("E2E_DISABLE_RATE_LIMIT", false)

	// Защита: E2E_DISABLE_RATE_LIMIT в проде выключил бы IP-rate-limit
	// для всех. Fail при APP_ENV=production.
	if cfg.IsProd() && cfg.DisableRateLimit {
		return nil, fmt.Errorf("E2E_DISABLE_RATE_LIMIT=true is not allowed in production")
	}

	cfg.JWT.AccessSecret = getEnv("JWT_ACCESS_SECRET", "dev_access_secret_change_me")
	cfg.JWT.RefreshSecret = getEnv("JWT_REFRESH_SECRET", "dev_refresh_secret_change_me")
	cfg.JWT.AccessTTL = getDuration("JWT_ACCESS_TTL", 15*time.Minute)
	cfg.JWT.RefreshTTL = getDuration("JWT_REFRESH_TTL", 30*24*time.Hour)

	// В проде запрещаем запускаться с дефолтными/короткими секретами —
	// иначе любой может подписать JWT с role=admin.
	if cfg.IsProd() {
		if err := validateProdSecrets(cfg); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(cfg.PublicBaseURL, "https://") {
			return nil, fmt.Errorf("PUBLIC_BASE_URL must start with https:// in production")
		}
	}

	cfg.SMTP.Host = getEnv("SMTP_HOST", "localhost")
	cfg.SMTP.Port = getEnv("SMTP_PORT", "1025")
	cfg.SMTP.User = os.Getenv("SMTP_USER")
	cfg.SMTP.Pass = os.Getenv("SMTP_PASS")
	cfg.SMTP.From = getEnv("SMTP_FROM", "noreply@namylimsya.local")
	cfg.SMTP.Enabled = getBool("SMTP_ENABLED", false)

	cfg.Storage.Driver = getEnv("STORAGE_DRIVER", "local")
	cfg.Storage.LocalPath = getEnv("STORAGE_LOCAL_PATH", "./storage")
	cfg.Storage.PublicBaseURL = getEnv("STORAGE_PUBLIC_BASE_URL", "http://localhost:"+cfg.HTTPPort)

	// Проверки прода — ПОСЛЕ инициализации всех полей (SMTP, storage).
	// SMTP обязателен в проде: иначе сброс пароля и дайджесты молча не работают —
	// отключённый отправитель возвращает nil как «успех» (см. email.Sender),
	// а UI при этом говорит «отправлено».
	if cfg.IsProd() {
		if err := validateProdSMTP(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func (c *Config) IsProd() bool { return c.AppEnv == "production" }

// defaultLogLevel — дефолтный уровень zerolog: debug в dev/тестах, info в проде,
// чтобы в production-логи не попадали SQL-трейсеры и debug-строки.
func defaultLogLevel(appEnv string) string {
	if appEnv == "production" {
		return "info"
	}
	return "debug"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func validateProdSecrets(cfg *Config) error {
	if len(cfg.JWT.AccessSecret) < 32 {
		return fmt.Errorf("JWT_ACCESS_SECRET must be at least 32 chars in production (got %d)", len(cfg.JWT.AccessSecret))
	}
	if len(cfg.JWT.RefreshSecret) < 32 {
		return fmt.Errorf("JWT_REFRESH_SECRET must be at least 32 chars in production (got %d)", len(cfg.JWT.RefreshSecret))
	}
	if cfg.JWT.AccessSecret == cfg.JWT.RefreshSecret {
		return fmt.Errorf("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET must differ")
	}
	if cfg.JWT.AccessSecret == "dev_access_secret_change_me" || cfg.JWT.RefreshSecret == "dev_refresh_secret_change_me" {
		return fmt.Errorf("default JWT secrets are not allowed in production")
	}
	return nil
}

// validateProdSMTP — SMTP обязателен в production. Без него сброс пароля и
// email-дайджесты молча не работают (отключённый отправитель возвращает nil).
func validateProdSMTP(cfg *Config) error {
	if !cfg.SMTP.Enabled {
		return fmt.Errorf("SMTP_ENABLED must be true in production (password reset and notification emails require it)")
	}
	return nil
}
