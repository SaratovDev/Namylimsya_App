package services

import (
	"context"
	"sync"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/rs/zerolog/log"
)

// EventWorker выполняет периодические «гигиенические» задачи:
//   - автозавершение прошедших событий (status=completed);
//   - снятие истёкших банов (is_banned=false для ban_expires_at < now()).
//
// Интервал — 5 минут; при shutdown останавливается через ctx.Done() и Stop().
type EventWorker struct {
	db *dbgen.Queries
	wg sync.WaitGroup
}

func NewEventWorker(db *dbgen.Queries) *EventWorker {
	return &EventWorker{db: db}
}

func (w *EventWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := w.db.MarkPastEventsCompleted(ctx); err != nil {
					log.Warn().Err(err).Msg("mark past events completed")
				}
				if err := w.db.UnbanExpiredUsers(ctx); err != nil {
					log.Warn().Err(err).Msg("unban expired users")
				}
			}
		}
	}()
}

func (w *EventWorker) Stop() { w.wg.Wait() }
