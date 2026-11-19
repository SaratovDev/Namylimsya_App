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

// RevokeAllForUser удаляет все refresh-токены пользователя (например, при бане).
func (s *SessionManager) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	pattern := fmt.Sprintf("refresh:%s:*", userID.String())
	iter := s.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return s.rdb.Del(ctx, keys...).Err()
}

const bannedKeyPrefix = "banned:"

func (s *SessionManager) MarkBanned(ctx context.Context, userID uuid.UUID, ttl time.Duration) error {
	return s.rdb.Set(ctx, bannedKeyPrefix+userID.String(), "1", ttl).Err()
}

func (s *SessionManager) IsBanned(ctx context.Context, userID uuid.UUID) bool {
	val, err := s.rdb.Get(ctx, bannedKeyPrefix+userID.String()).Result()
	return err == nil && val == "1"
}

func (s *SessionManager) ClearBanned(ctx context.Context, userID uuid.UUID) error {
	return s.rdb.Del(ctx, bannedKeyPrefix+userID.String()).Err()
}
