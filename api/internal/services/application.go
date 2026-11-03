package services

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/google/uuid"
)

var (
	ErrCannotApplyToOwnEvent = errors.New("cannot apply to your own event")
	ErrApplicationExists     = errors.New("application already exists for this event")
	ErrTooManyApplications   = errors.New("cannot have more than 3 active applications")
	ErrEventFull             = errors.New("event is full")
	ErrEventNotActive        = errors.New("event is not active")
	ErrNotApplicationOwner   = errors.New("you cannot modify this application")
)

type ApplicationService struct {
	db    *dbgen.Queries
	notif *NotificationService
}

func NewApplicationService(db *dbgen.Queries, notif *NotificationService) *ApplicationService {
	return &ApplicationService{db: db, notif: notif}
}

func (s *ApplicationService) Apply(ctx context.Context, eventID, userID uuid.UUID) (*dbgen.Application, error) {
	event, err := s.db.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event.OrganizerID == userID {
		return nil, ErrCannotApplyToOwnEvent
	}
	if event.Status != "active" {
		return nil, ErrEventNotActive
	}

	// Уже есть заявка?
	if _, err := s.db.GetUserApplicationForEvent(ctx, dbgen.GetUserApplicationForEventParams{
		EventID: eventID, UserID: userID,
	}); err == nil {
		return nil, ErrApplicationExists
	}

	// Лимит активных заявок
	count, err := s.db.CountActiveApplicationsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if count >= 3 {
		return nil, ErrTooManyApplications
	}

	app, err := s.db.CreateApplication(ctx, dbgen.CreateApplicationParams{
		EventID: eventID, UserID: userID,
	})
	if err != nil {
		return nil, err
	}

	// Уведомляем организатора о новой заявке
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

	return &app, nil
}

func (s *ApplicationService) Approve(ctx context.Context, applicationID, organizerID uuid.UUID) (*dbgen.Application, error) {
	app, err := s.db.GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	event, err := s.db.GetEventByID(ctx, app.EventID)
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

	// Проверка лимита участников
	approved, err := s.db.CountApprovedApplicationsForEvent(ctx, app.EventID)
	if err != nil {
		return nil, err
	}
	if event.ParticipantLimit.Valid && approved >= int64(event.ParticipantLimit.Int32) {
		return nil, ErrEventFull
	}

	updated, err := s.db.UpdateApplicationStatus(ctx, dbgen.UpdateApplicationStatusParams{
		ID:     applicationID,
		Status: "approved",
	})
	if err != nil {
		return nil, err
	}

	// Уведомляем участника об одобрении
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
	event, err := s.db.GetEventByID(ctx, app.EventID)
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
		ID:     applicationID,
		Status: "rejected",
	})
	if err != nil {
		return nil, err
	}

	// Уведомляем участника об отказе
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
	})
	if err != nil {
		return nil, err
	}

	// Уведомляем организатора об отмене заявки участником
	if s.notif != nil {
		event, err := s.db.GetEventByID(ctx, updated.EventID)
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
	event, err := s.db.GetEventByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if event.OrganizerID != organizerID {
		return nil, ErrNotApplicationOwner
	}
	return s.db.ListEventApplications(ctx, eventID)
}

// CanSeeExactLocation — может ли пользователь видеть точное место
func (s *ApplicationService) CanSeeExactLocation(ctx context.Context, eventID uuid.UUID, userID *uuid.UUID) (bool, error) {
	event, err := s.db.GetEventByID(ctx, eventID)
	if err != nil {
		return false, err
	}
	if !event.HideExactLocation {
		return true, nil // место не скрыто — видят все
	}
	if userID == nil {
		return false, nil // гость — не видит
	}
	if event.OrganizerID == *userID {
		return true, nil // организатор — видит всегда
	}
	app, err := s.db.GetUserApplicationForEvent(ctx, dbgen.GetUserApplicationForEventParams{
		EventID: eventID, UserID: *userID,
	})
	if err != nil {
		return false, nil
	}
	return app.Status == "approved", nil
}
