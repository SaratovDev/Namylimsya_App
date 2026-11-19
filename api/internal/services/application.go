package services

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
)

var (
	ErrCannotApplyToOwnEvent = errors.New("Это ваше событие, зачем на него заявляться?")
	ErrApplicationExists     = errors.New("Сюда уже отправлена заяка, вторая не нужна")
	ErrTooManyApplications   = errors.New("Больше трех заявок отправить не получится")
	ErrEventFull             = errors.New("Тут уже хватает участников")
	ErrEventNotActive        = errors.New("Событие не активно")
	ErrEventHidden           = errors.New("Событие недоступно")
	ErrNotApplicationOwner   = errors.New("Нельзя")
)

type ApplicationService struct {
	db    *dbgen.Queries
	pool  *pgxpool.Pool
	notif *NotificationService
}

func NewApplicationService(db *dbgen.Queries, pool *pgxpool.Pool, notif *NotificationService) *ApplicationService {
	return &ApplicationService{db: db, pool: pool, notif: notif}
}

// advisoryLock берёт транзакционную advisory-блокировку по строковому ключу.
// Снимается автоматически при Commit/Rollback.
func advisoryLock(ctx context.Context, tx pgx.Tx, key string) error {
	_, err := tx.Exec(ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))", key)
	return err
}

func (s *ApplicationService) Apply(ctx context.Context, eventID, userID uuid.UUID) (*dbgen.Application, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := dbgen.New(tx)

	// 1. Блокировка события: не даст подать заявку на отменённое/прошедшее.
	event, err := qtx.GetEventForUpdate(ctx, eventID)
	if err != nil {
		return nil, err
	}

	if event.OrganizerID == userID {
		return nil, ErrCannotApplyToOwnEvent
	}
	if event.Hidden {
		return nil, ErrEventHidden
	}
	if event.Status != "active" {
		return nil, ErrEventNotActive
	}
	if event.StartsAt.Before(time.Now()) {
		return nil, ErrEventInPast
	}

	// 2. Advisory lock на пользователя — сериализует все Apply одного юзера.
	// Без этого 4 параллельных Apply на разные события пройдут проверку лимита.
	// Порядок блокировок: event → user. У Approve — только event. Без deadlock.
	if err := advisoryLock(ctx, tx, "apply:user:"+userID.String()); err != nil {
		return nil, err
	}

	var app *dbgen.Application

	existing, err := qtx.GetUserApplicationForEvent(ctx, dbgen.GetUserApplicationForEventParams{
		EventID: eventID, UserID: userID,
	})

	switch {
	case err == nil:
		// Заявка уже есть
		if existing.Status == "pending" || existing.Status == "approved" {
			return nil, ErrApplicationExists
		}
		// Cancelled/rejected — реактивируем, но лимит проверяем ЗАНОВО.
		count, err := qtx.CountActiveApplicationsByUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		if count >= 3 {
			return nil, ErrTooManyApplications
		}
		updated, err := qtx.UpdateApplicationStatus(ctx, dbgen.UpdateApplicationStatusParams{
			ID:           existing.ID,
			Status:       "pending",
			CancelReason: nil,
			AllowedFrom:  []string{"cancelled", "rejected"},
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Статус изменился параллельно (кто-то уже реактивировал
				// или подал заново).
				return nil, ErrApplicationExists
			}
			return nil, err
		}
		app = &updated

	case errors.Is(err, pgx.ErrNoRows):
		// Заявки нет — проверяем лимит и создаём
		count, err := qtx.CountActiveApplicationsByUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		if count >= 3 {
			return nil, ErrTooManyApplications
		}
		created, err := qtx.CreateApplication(ctx, dbgen.CreateApplicationParams{
			EventID: eventID, UserID: userID,
		})
		if err != nil {
			// Race: кто-то успел вставить ту же заявку параллельно
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return nil, ErrApplicationExists
			}
			return nil, err
		}
		app = &created

	default:
		// Реальная ошибка БД (не ErrNoRows) — не маскируем
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Уведомление — после коммита, чтобы при откате не было фантомных писем
	if s.notif != nil {
		if _, err := s.notif.Create(ctx, CreateInput{
			UserID: event.OrganizerID,
			Type:   "application_new",
			Payload: map[string]any{
				"event_id":       eventID,
				"event_title":    event.Title,
				"applicant_id":   userID,
				"application_id": app.ID,
			},
			SendEmail: true,
		}); err != nil {
			log.Warn().Err(err).
				Str("type", "application_new").
				Str("event_id", eventID.String()).
				Str("application_id", app.ID.String()).
				Msg("failed to create notification")
		}
	}

	return app, nil
}

func (s *ApplicationService) Approve(ctx context.Context, applicationID, organizerID uuid.UUID) (*dbgen.Application, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := dbgen.New(tx)

	// Первое чтение — только чтобы узнать event_id для advisory lock.
	app, err := qtx.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}

	// Advisory lock на event сериализует все Approve этого события.
	// Без этого два параллельных Approve могут превысить participant_limit.
	if err := advisoryLock(ctx, tx, "approve:event:"+app.EventID.String()); err != nil {
		return nil, err
	}

	// ПЕРЕЧИТАТЬ app после lock: пока ждали блокировку, другой процесс мог
	// отменить/отклонить заявку. Без перечитывания мы бы одобрили
	// cancelled-заявку, опираясь на устаревший app.Status из памяти.
	app, err = qtx.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}

	event, err := qtx.GetEventForUpdate(ctx, app.EventID)
	if err != nil {
		return nil, err
	}
	if event.OrganizerID != organizerID {
		return nil, ErrNotApplicationOwner
	}
	if event.Status != "active" {
		return nil, ErrEventNotActive
	}
	if app.Status != "pending" {
		return nil, errors.New("application is not pending")
	}

	approved, err := qtx.CountApprovedApplicationsForEvent(ctx, app.EventID)
	if err != nil {
		return nil, err
	}
	if event.ParticipantLimit.Valid && approved >= int64(event.ParticipantLimit.Int32) {
		return nil, ErrEventFull
	}

	updated, err := qtx.UpdateApplicationStatus(ctx, dbgen.UpdateApplicationStatusParams{
		ID:          applicationID,
		Status:      "approved",
		AllowedFrom: []string{"pending"},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Статус изменился между перечитыванием и UPDATE (редкий, но
			// возможный случай: Reject без lock).
			return nil, errors.New("application is not pending")
		}
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if s.notif != nil {
		if _, err := s.notif.Create(ctx, CreateInput{
			UserID: updated.UserID,
			Type:   "application_approved",
			Payload: map[string]any{
				"event_id":       updated.EventID,
				"application_id": updated.ID,
			},
			SendEmail: true,
		}); err != nil {
			log.Warn().Err(err).
				Str("type", "application_approved").
				Str("event_id", updated.EventID.String()).
				Str("application_id", updated.ID.String()).
				Msg("failed to create notification")
		}
	}

	return &updated, nil
}

func (s *ApplicationService) Reject(ctx context.Context, applicationID, organizerID uuid.UUID) (*dbgen.Application, error) {
	app, err := s.db.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	event, err := s.db.GetEventByID(ctx, dbgen.GetEventByIDParams{ID: app.EventID})
	if err != nil {
		return nil, err
	}
	if event.OrganizerID != organizerID {
		return nil, ErrNotApplicationOwner
	}
	if app.Status != "pending" {
		return nil, errors.New("application is not pending")
	}

	updated, err := s.db.UpdateApplicationStatus(ctx, dbgen.UpdateApplicationStatusParams{
		ID:          applicationID,
		Status:      "rejected",
		AllowedFrom: []string{"pending"},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("application is not pending")
		}
		return nil, err
	}

	if s.notif != nil {
		if _, err := s.notif.Create(ctx, CreateInput{
			UserID: updated.UserID,
			Type:   "application_rejected",
			Payload: map[string]any{
				"event_id":       updated.EventID,
				"application_id": updated.ID,
			},
			SendEmail: true,
		}); err != nil {
			log.Warn().Err(err).
				Str("type", "application_rejected").
				Str("event_id", updated.EventID.String()).
				Str("application_id", updated.ID.String()).
				Msg("failed to create notification")
		}
	}

	return &updated, nil
}

func (s *ApplicationService) Cancel(ctx context.Context, applicationID, userID uuid.UUID, reason string) (*dbgen.Application, error) {
	app, err := s.db.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	if app.UserID != userID {
		return nil, ErrNotApplicationOwner
	}
	if app.Status != "pending" && app.Status != "approved" {
		return nil, errors.New("cannot cancel application in current state")
	}

	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	updated, err := s.db.UpdateApplicationStatus(ctx, dbgen.UpdateApplicationStatusParams{
		ID:           applicationID,
		Status:       "cancelled",
		CancelReason: reasonPtr,
		AllowedFrom:  []string{"pending", "approved"},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("cannot cancel application in current state")
		}
		return nil, err
	}

	if s.notif != nil {
		event, err := s.db.GetEventByID(ctx, dbgen.GetEventByIDParams{ID: updated.EventID})
		if err == nil {
			payload := map[string]any{
				"event_id":       updated.EventID,
				"application_id": updated.ID,
				"applicant_id":   updated.UserID,
			}
			if reason != "" {
				payload["reason"] = reason
			}
			if _, err := s.notif.Create(ctx, CreateInput{
				UserID:    event.OrganizerID,
				Type:      "application_cancelled",
				Payload:   payload,
				SendEmail: true,
			}); err != nil {
				log.Warn().Err(err).
					Str("type", "application_cancelled").
					Str("event_id", updated.EventID.String()).
					Str("application_id", updated.ID.String()).
					Msg("failed to create notification")
			}
		} else {
			log.Warn().Err(err).
				Str("event_id", updated.EventID.String()).
				Msg("failed to load event for cancellation notification")
		}
	}

	return &updated, nil
}

func (s *ApplicationService) ListForEvent(ctx context.Context, eventID, organizerID uuid.UUID) ([]dbgen.ListEventApplicationsRow, error) {
	event, err := s.db.GetEventByID(ctx, dbgen.GetEventByIDParams{ID: eventID})
	if err != nil {
		return nil, err
	}
	if event.OrganizerID != organizerID {
		return nil, ErrNotApplicationOwner
	}
	return s.db.ListEventApplications(ctx, eventID)
}

func (s *ApplicationService) ListByUser(ctx context.Context, userID uuid.UUID) ([]dbgen.ListApplicationsByUserRow, error) {
	return s.db.ListApplicationsByUser(ctx, userID)
}
