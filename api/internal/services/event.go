package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/categories"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

var (
	ErrTooManyActiveEvents     = errors.New("Больше двух активных событий не создать")
	ErrEventInPast             = errors.New("Машину времени ещё не изобрели: событие должно быть в будущем")
	ErrNotEventOwner           = errors.New("Это не твоё событие, менять может только организатор")
	ErrTooLateToEdit           = errors.New("Поздновато спохватился. Время и место нельзя менять меньше чем за 24 часа до события")
	ErrInvalidParticipantLimit = errors.New("Лимит участников должен быть больше нуля. Событие без людей это грустно")
	ErrTitleRequired           = errors.New("Без названия никак,  придумай заголовок")
	ErrTitleTooLong            = errors.New("Заголовок слишком длинный — уложись в 100 символов")
	ErrDescriptionTooLong      = errors.New("Описание разрослось. Максимум 5000 символов")
	ErrInvalidCategory         = errors.New("Неизвестная категория. Выберите из списка")
	ErrEventNotFound           = errors.New("Ой. А такого события нет.")

	// Update-специфичные:
	ErrCannotEditInactive     = errors.New("Редактировать можно только активное событие")
	ErrParticipantLimitTooLow = errors.New("Лимит не может быть меньше числа уже одобренных участников")

	// Координаты:
	ErrInvalidLatitude    = errors.New("Широта должна быть в диапазоне [-90, 90]")
	ErrInvalidLongitude   = errors.New("Долгота должна быть в диапазоне [-180, 180]")
	ErrCoordinatesMissing = errors.New("Укажите координаты события (lat/lng)")
)

type EventService struct {
	db    *dbgen.Queries
	pool  *pgxpool.Pool
	notif *NotificationService
}

func NewEventService(db *dbgen.Queries, pool *pgxpool.Pool, notif *NotificationService) *EventService {
	return &EventService{db: db, pool: pool, notif: notif}
}

// Вспомогательные функции для конвертации
func toFloat8(f float64) pgtype.Float8 {
	return pgtype.Float8{Float64: f, Valid: true}
}

func toInt4(i *int32) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: *i, Valid: true}
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
	Role              string
}

func (s *EventService) Create(ctx context.Context, input CreateEventInput) (*dbgen.Event, error) {
	normalizedCat, err := validateEventInput(input.Title, input.Category, input.Description, input.ParticipantLimit)
	if err != nil {
		return nil, err
	}

	if err := validateCoordinates(input.Lat, input.Lng); err != nil {
		return nil, err
	}

	if input.StartsAt.Before(time.Now()) {
		return nil, ErrEventInPast
	}

	if input.Role != "admin" && input.Role != "moderator" {
		count, err := s.db.CountActiveEventsByOrganizer(ctx, input.OrganizerID)
		if err != nil {
			return nil, err
		}
		if count >= 2 {
			return nil, ErrTooManyActiveEvents
		}
	}

	var desc *string
	if input.Description != "" {
		desc = &input.Description
	}
	var address *string
	if input.Address != "" {
		address = &input.Address
	}

	event, err := s.db.CreateEvent(ctx, dbgen.CreateEventParams{
		OrganizerID:       input.OrganizerID,
		Title:             input.Title,
		Category:          normalizedCat,
		Description:       desc,
		StartsAt:          input.StartsAt,
		City:              input.City,
		Address:           address,
		Lat:               toFloat8(input.Lat),
		Lng:               toFloat8(input.Lng),
		HideExactLocation: input.HideExactLocation,
		ParticipantLimit:  toInt4(input.ParticipantLimit),
	})
	if err != nil {
		return nil, err
	}
	return &event, nil
}

type UpdateEventInput struct {
	ID                uuid.UUID
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

func (s *EventService) Update(ctx context.Context, input UpdateEventInput) (*dbgen.Event, error) {
	normalizedCat, err := validateEventInput(input.Title, input.Category, input.Description, input.ParticipantLimit)
	if err != nil {
		return nil, err
	}

	if err := validateCoordinates(input.Lat, input.Lng); err != nil {
		return nil, err
	}

	current, err := s.db.GetEventByID(ctx, dbgen.GetEventByIDParams{
		ID: input.ID,
	})
	if err != nil {
		return nil, err
	}

	if current.OrganizerID != input.OrganizerID {
		return nil, ErrNotEventOwner
	}

	// Редактировать можно только активное событие.
	if current.Status != "active" {
		return nil, ErrCannotEditInactive
	}

	// starts_at в прошлое — нельзя (аналогично Create). Иначе организатор
	// может сдвинуть событие «назад», и участники получат уведомления о
	// том, что уже прошло.
	if input.StartsAt.Before(time.Now()) {
		return nil, ErrEventInPast
	}

	// participant_limit ниже approved — нельзя. Иначе «лишние» approved
	// участники остаются с подтверждённой заявкой, но не влезают в лимит.
	if input.ParticipantLimit != nil {
		approved, err := s.db.CountApprovedApplicationsForEvent(ctx, input.ID)
		if err != nil {
			return nil, err
		}
		if approved > int64(*input.ParticipantLimit) {
			return nil, ErrParticipantLimitTooLow
		}
	}

	// Проверка безопасности: если до события < 24 часов, запрещаем менять время и место
	timeUntilEvent := time.Until(current.StartsAt)
	if timeUntilEvent < 24*time.Hour {
		timeChanged := !current.StartsAt.Equal(input.StartsAt)
		locChanged := current.Lat.Float64 != input.Lat || current.Lng.Float64 != input.Lng
		if timeChanged || locChanged {
			return nil, ErrTooLateToEdit
		}
	}

	var desc *string
	if input.Description != "" {
		desc = &input.Description
	}
	var address *string
	if input.Address != "" {
		address = &input.Address
	}

	updated, err := s.db.UpdateEvent(ctx, dbgen.UpdateEventParams{
		ID:                input.ID,
		Title:             input.Title,
		Category:          normalizedCat,
		Description:       desc,
		StartsAt:          input.StartsAt,
		City:              input.City,
		Address:           address,
		Lat:               toFloat8(input.Lat),
		Lng:               toFloat8(input.Lng),
		HideExactLocation: input.HideExactLocation,
		ParticipantLimit:  toInt4(input.ParticipantLimit),
		OrganizerID:       input.OrganizerID,
	})
	if err != nil {
		return nil, fmt.Errorf("update event: %w", err)
	}
	return &updated, nil
}

type FeedFilters struct {
	Category   *string
	City       *string
	DateFrom   *time.Time
	DateTo     *time.Time
	CursorTime *time.Time
	CursorID   *uuid.UUID
	ViewerID   *uuid.UUID
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
		CursorID:   f.CursorID,
		ViewerID:   f.ViewerID,
	})
}

type ArchiveFilters struct {
	Category   *string
	City       *string
	DateFrom   *time.Time
	DateTo     *time.Time
	CursorTime *time.Time
	CursorID   *uuid.UUID
	ViewerID   *uuid.UUID
	Limit      int32
}

func (s *EventService) GetArchive(ctx context.Context, f ArchiveFilters) ([]dbgen.GetEventsArchiveRow, error) {
	if f.Limit <= 0 || f.Limit > 50 {
		f.Limit = 20
	}

	return s.db.GetEventsArchive(ctx, dbgen.GetEventsArchiveParams{
		Limit:      f.Limit,
		Category:   f.Category,
		City:       f.City,
		DateFrom:   f.DateFrom,
		DateTo:     f.DateTo,
		CursorTime: f.CursorTime,
		CursorID:   f.CursorID,
		ViewerID:   f.ViewerID,
	})
}

func (s *EventService) GetByID(ctx context.Context, id uuid.UUID, viewerID *uuid.UUID) (*dbgen.GetEventByIDRow, error) {
	event, err := s.db.GetEventByID(ctx, dbgen.GetEventByIDParams{
		ID:       id,
		ViewerID: viewerID,
	})
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *EventService) Cancel(ctx context.Context, eventID, organizerID uuid.UUID, reason string) (*dbgen.Event, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := dbgen.New(tx)

	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	event, err := qtx.CancelEvent(ctx, dbgen.CancelEventParams{
		ID:           eventID,
		CancelReason: reasonPtr,
		OrganizerID:  organizerID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// CancelEvent вернул 0 строк — различаем причину.
			current, getErr := s.db.GetEventByID(ctx, dbgen.GetEventByIDParams{ID: eventID})
			if getErr != nil {
				return nil, ErrEventNotFound
			}
			if current.OrganizerID != organizerID {
				return nil, ErrNotEventOwner
			}
			return nil, ErrEventNotActive
		}
		return nil, err
	}

	// Собираем затронутых (pending + approved) до отмены — внутри той же транзакции.
	activeApps, err := qtx.ListActiveApplicationsForEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}

	// Каскадно отменяем pending и approved заявки.
	if err := qtx.CancelAllActiveApplicationsForEvent(ctx, dbgen.CancelAllActiveApplicationsForEventParams{
		EventID:      eventID,
		CancelReason: reasonPtr,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Уведомления — после коммита, чтобы не отправлять фантомные письма при откате.
	if s.notif != nil {
		for _, app := range activeApps {
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
					Str("event_id", eventID.String()).
					Str("application_id", app.ID.String()).
					Msg("failed to notify cancelled application")
			}
		}
	}

	return &event, nil
}

// validateEventInput проверяет поля и возвращает нормализованную категорию.
func validateEventInput(title, category, description string, participantLimit *int32) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", ErrTitleRequired
	}
	if len([]rune(title)) > 100 {
		return "", ErrTitleTooLong
	}
	if len([]rune(description)) > 5000 {
		return "", ErrDescriptionTooLong
	}
	if participantLimit != nil && *participantLimit < 1 {
		return "", ErrInvalidParticipantLimit
	}
	normalizedCat, ok := categories.Normalize(category)
	if !ok {
		return "", ErrInvalidCategory
	}
	return normalizedCat, nil
}

func (s *EventService) ListMyOrganized(ctx context.Context, orgID uuid.UUID) ([]dbgen.GetMyOrganizedEventsRow, error) {
	return s.db.GetMyOrganizedEvents(ctx, orgID)
}

// validateCoordinates проверяет диапазоны широты/долготы и ловит
// классическую ошибку «координаты не заданы» (0, 0 — точка в Атлантике).
// Без этого валидатора событие с lat=999 или (0,0) молча сохранялось
// и приводило к точке «вне Земли» или к пустой точке на карте.
func validateCoordinates(lat, lng float64) error {
	if lat < -90 || lat > 90 {
		return ErrInvalidLatitude
	}
	if lng < -180 || lng > 180 {
		return ErrInvalidLongitude
	}
	if lat == 0 && lng == 0 {
		return ErrCoordinatesMissing
	}
	return nil
}

// ListParticipants — approved-участники события (орг первым).
// Pending/rejected/cancelled не возвращаются. Время/возраст/город —
// из связанных записей users.
func (s *EventService) ListParticipants(ctx context.Context, eventID uuid.UUID) ([]dbgen.ListApprovedParticipantsRow, error) {
	return s.db.ListApprovedParticipants(ctx, eventID)
}
