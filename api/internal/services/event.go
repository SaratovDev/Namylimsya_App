package services

import (
	"context"
	"errors"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

var (
	ErrTooManyActiveEvents = errors.New("cannot have more than 2 active events")
	ErrEventInPast         = errors.New("event must be in the future")
	ErrNotEventOwner       = errors.New("only organizer can cancel this event")
)

type EventService struct {
	db    *dbgen.Queries
	notif *NotificationService
}

func NewEventService(db *dbgen.Queries, notif *NotificationService) *EventService {
	return &EventService{db: db, notif: notif}
}

type CreateEventInput struct {
	OrganizerID       uuid.UUID
	Title             string
	Category          string
	Description       string
	StartsAt          time.Time
	City              string
	Address           string
	Lat               float64
	Lng               float64
	HideExactLocation bool
	ParticipantLimit  *int32
}

func (s *EventService) Create(ctx context.Context, input CreateEventInput) (*dbgen.Event, error) {
	if input.StartsAt.Before(time.Now()) {
		return nil, ErrEventInPast
	}

	count, err := s.db.CountActiveEventsByOrganizer(ctx, input.OrganizerID)
	if err != nil {
		return nil, err
	}
	if count >= 2 {
		return nil, ErrTooManyActiveEvents
	}

	var desc *string
	if input.Description != "" {
		desc = &input.Description
	}
	var address *string
	if input.Address != "" {
		address = &input.Address
	}

	// pgtype для Lat/Lng/ParticipantLimit
	lat := pgtype.Float8{Float64: input.Lat, Valid: true}
	lng := pgtype.Float8{Float64: input.Lng, Valid: true}
	var limit pgtype.Int4
	if input.ParticipantLimit != nil {
		limit = pgtype.Int4{Int32: *input.ParticipantLimit, Valid: true}
	}

	event, err := s.db.CreateEvent(ctx, dbgen.CreateEventParams{
		OrganizerID:       input.OrganizerID,
		Title:             input.Title,
		Category:          input.Category,
		Description:       desc,
		StartsAt:          input.StartsAt, // time.Time
		City:              input.City,
		Address:           address,
		Lat:               lat,
		Lng:               lng,
		HideExactLocation: input.HideExactLocation,
		ParticipantLimit:  limit,
	})
	if err != nil {
		return nil, err
	}
	return &event, nil
}

type FeedFilters struct {
	Category   *string
	City       *string
	DateFrom   *time.Time
	DateTo     *time.Time
	CursorTime *time.Time
	Limit      int32
}

func (s *EventService) GetFeed(ctx context.Context, f FeedFilters) ([]dbgen.GetEventsFeedRow, error) {
	if f.Limit <= 0 || f.Limit > 50 {
		f.Limit = 20
	}

	return s.db.GetEventsFeed(ctx, dbgen.GetEventsFeedParams{
		Limit:      f.Limit,
		Category:   f.Category,
		City:       f.City,
		DateFrom:   f.DateFrom,
		DateTo:     f.DateTo,
		CursorTime: f.CursorTime,
	})
}

func (s *EventService) GetByID(ctx context.Context, id uuid.UUID) (*dbgen.Event, error) {
	event, err := s.db.GetEventByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *EventService) Cancel(ctx context.Context, eventID, organizerID uuid.UUID, reason string) (*dbgen.Event, error) {
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	event, err := s.db.CancelEvent(ctx, dbgen.CancelEventParams{
		ID:           eventID,
		CancelReason: reasonPtr,
		OrganizerID:  organizerID,
	})
	if err != nil {
		return nil, ErrNotEventOwner
	}

	// Уведомляем всех одобренных участников об отмене
	if s.notif != nil {
		approvedApps, err := s.db.ListApprovedForEvent(ctx, eventID)
		if err != nil {
			log.Warn().Err(err).
				Str("event_id", eventID.String()).
				Msg("failed to load approved applications for cancellation notification")
		} else {
			for _, app := range approvedApps {
				payload := map[string]any{
					"event_id":       eventID,
					"event_title":    event.Title,
					"application_id": app.ID,
				}
				if reason != "" {
					payload["reason"] = reason
				}
				if _, err := s.notif.Create(ctx, CreateInput{
					UserID:    app.UserID,
					Type:      "event_cancelled",
					Payload:   payload,
					SendEmail: true,
				}); err != nil {
					log.Warn().Err(err).
						Str("type", "event_cancelled").
						Str("event_id", eventID.String()).
						Str("user_id", app.UserID.String()).
						Msg("failed to create cancellation notification")
				}
			}
		}
	}

	return &event, nil
}
