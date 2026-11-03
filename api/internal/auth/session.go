package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type SessionManager struct {
	rdb *redis.Client
}

func NewSessionManager(rdb *redis.Client) *SessionManager {
	return &SessionManager{rdb: rdb}
}

// Сохраняем refresh токен. Ключ: refresh:<userID>:<jti>
func (s *SessionManager) SaveRefreshToken(ctx context.Context, userID uuid.UUID, jti string, ttl time.Duration) error {
	key := fmt.Sprintf("refresh:%s:%s", userID.String(), jti)
	return s.rdb.Set(ctx, key, "valid", ttl).Err()
}

// Проверяем, существует ли токен (не отозван)
func (s *SessionManager) IsRefreshTokenValid(ctx context.Context, userID uuid.UUID, jti string) (bool, error) {
	key := fmt.Sprintf("refresh:%s:%s", userID.String(), jti)
	val, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil // токен отозван или истек
	}
	if err != nil {
		return false, err
	}
	return val == "valid", nil
}

// Удаляем токен (логаут)
func (s *SessionManager) RevokeRefreshToken(ctx context.Context, userID uuid.UUID, jti string) error {
	key := fmt.Sprintf("refresh:%s:%s", userID.String(), jti)
	return s.rdb.Del(ctx, key).Err()
}
