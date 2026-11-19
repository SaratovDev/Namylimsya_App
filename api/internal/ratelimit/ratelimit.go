package ratelimit

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	rdb *redis.Client
	seq atomic.Uint64 // уникализатор member: параллельные запросы в один и тот же ns
}

func New(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb}
}

// Allow — true если в течение window сделано меньше limit действий.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	redisKey := fmt.Sprintf("rl:%s", key)
	now := time.Now().UnixNano()
	windowStart := now - int64(window)

	// Member в ZSET уникален. Без unique-суффикса два параллельных запроса
	// с одинаковым now (coarse clocksource в VM, multi-core) схлопываются
	// в одну запись — undercount, rate-limit fires late или never.
	member := fmt.Sprintf("%d-%d", now, l.seq.Add(1))

	pipe := l.rdb.TxPipeline()
	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	pipe.ZCard(ctx, redisKey)
	pipe.ZAdd(ctx, redisKey, redis.Z{Score: float64(now), Member: member})
	pipe.Expire(ctx, redisKey, window*2)
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	count := cmds[1].(*redis.IntCmd).Val()
	return count < int64(limit), nil
}
