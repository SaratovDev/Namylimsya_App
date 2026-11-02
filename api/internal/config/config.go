package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv   string
	HTTPPort string

	DatabaseURL string
	RedisURL    string

	JWT struct {
		AccessSecret  string
		RefreshSecret string
		AccessTTL     time.Duration
		RefreshTTL    time.Duration
	}

	SMTP struct {
		Host                    string
		Port                    string
		User                    string
		Pass                    string
		From                    string
		EmailVerificationEnabled bool
	}

	Storage struct {
		Driver    string // "local" | "s3"
		LocalPath string
	}

	YandexMapsAPIKey string
}

func Load() (*Config, error) {
	// .env в корне проекта; ищем в текущей, затем в родительских папках.
	for _, p := range []string{".env", "../.env", "../../.env"} {
		_ = godotenv.Load(p)
	}

	cfg := &Config{
		AppEnv:      getEnv("APP_ENV", "development"),
		HTTPPort:    getEnv("HTTP_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		YandexMapsAPIKey: os.Getenv("YANDEX_MAPS_API_KEY"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is required")
	}

	cfg.JWT.AccessSecret = getEnv("JWT_ACCESS_SECRET", "dev_access_secret_change_me")
	cfg.JWT.RefreshSecret = getEnv("JWT_REFRESH_SECRET", "dev_refresh_secret_change_me")
	cfg.JWT.AccessTTL = getDuration("JWT_ACCESS_TTL", 15*time.Minute)
	cfg.JWT.RefreshTTL = getDuration("JWT_REFRESH_TTL", 30*24*time.Hour)

	cfg.SMTP.Host = getEnv("SMTP_HOST", "localhost")
	cfg.SMTP.Port = getEnv("SMTP_PORT", "1025")
	cfg.SMTP.User = os.Getenv("SMTP_USER")
	cfg.SMTP.Pass = os.Getenv("SMTP_PASS")
	cfg.SMTP.From = getEnv("SMTP_FROM", "noreply@namylimsya.local")
	cfg.SMTP.EmailVerificationEnabled = getBool("EMAIL_VERIFICATION_ENABLED", false)

	cfg.Storage.Driver = getEnv("STORAGE_DRIVER", "local")
	cfg.Storage.LocalPath = getEnv("STORAGE_LOCAL_PATH", "./storage")

	return cfg, nil
}

func (c *Config) IsProd() bool { return c.AppEnv == "production" }

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