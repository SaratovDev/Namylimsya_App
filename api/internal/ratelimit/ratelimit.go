package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb}
}

// Allow — true если в течение window сделано меньше limit действий.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	redisKey := fmt.Sprintf("rl:%s", key)
	now := time.Now().UnixNano()
	windowStart := now - int64(window)

	pipe := l.rdb.TxPipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	pipe.ZCard(ctx, redisKey)
	pipe.ZAdd(ctx, redisKey, redis.Z{Score: float64(now), Member: now})
	pipe.Expire(ctx, redisKey, window*2)
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	count := cmds[1].(*redis.IntCmd).Val()
	return count < int64(limit), nil
}
