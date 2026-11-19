package handlers

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/SaratovDev/Namylimsya_App/api/internal/categories"
	"github.com/SaratovDev/Namylimsya_App/api/internal/httpx"
	"github.com/SaratovDev/Namylimsya_App/api/internal/middleware"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
)

type EventHandler struct {
	svc    *services.EventService
	appSvc *services.ApplicationService
}

func NewEventHandler(svc *services.EventService, appSvc *services.ApplicationService) *EventHandler {
	return &EventHandler{svc: svc, appSvc: appSvc}
}

func (h *EventHandler) GetFeed(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := int32(20)
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = int32(parsed)
		}
	}

	filters := services.FeedFilters{Limit: limit}

	if raw := q.Get("category"); raw != "" {
		parts := strings.Split(raw, ",")
		normalized := make([]string, 0, len(parts))
		for _, p := range parts {
			if n, ok := categories.Normalize(p); ok {
				normalized = append(normalized, n)
			}
		}
		if len(normalized) == 0 {
			httpx.OK(w, map[string]any{
				"events":      []any{},
				"next_cursor": nil,
			})
			return
		}
		joined := strings.Join(normalized, ",")
		filters.Category = &joined
	}
	if c := q.Get("city"); c != "" {
		filters.City = &c
	}
	if df := q.Get("date_from"); df != "" {
		if t, err := time.Parse(time.RFC3339, df); err == nil {
			filters.DateFrom = &t
		}
	}
	if dt := q.Get("date_to"); dt != "" {
		if t, err := time.Parse(time.RFC3339, dt); err == nil {
			filters.DateTo = &t
		}
	}
	// Приоритет: ?cursor= (новый композитный). Fallback: ?cursor_time=
	// (старый формат, только starts_at — если клиент ещё не обновлён).
	if cur := q.Get("cursor"); cur != "" {
		filters.CursorTime, filters.CursorID = decodeCursor(cur)
	} else if cur := q.Get("cursor_time"); cur != "" {
		if t, err := time.Parse(time.RFC3339, cur); err == nil {
			filters.CursorTime = &t
		}
	}

	if uid, ok := middleware.GetUserID(r.Context()); ok {
		filters.ViewerID = &uid
	}

	events, err := h.svc.GetFeed(r.Context(), filters)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	for i := range events {
		e := &events[i]
		if e.CanSeeExactLocation {
			continue
		}
		maskLatLng(e.ID, &e.Lat, &e.Lng)
		e.Address = nil
	}

	var nextCursor *string
	if len(events) > 0 {
		last := events[len(events)-1]
		c := encodeCursor(last.StartsAt, last.ID)
		nextCursor = &c
	}

	httpx.OK(w, map[string]any{
		"events":      events,
		"next_cursor": nextCursor,
	})
}

func (h *EventHandler) GetArchive(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := int32(20)
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 50 {
			limit = int32(parsed)
		}
	}

	filters := services.ArchiveFilters{Limit: limit}

	// Категория — как в ленте: split по запятой, нормализация, join.
	if raw := q.Get("category"); raw != "" {
		parts := strings.Split(raw, ",")
		normalized := make([]string, 0, len(parts))
		for _, p := range parts {
			if n, ok := categories.Normalize(p); ok {
				normalized = append(normalized, n)
			}
		}
		if len(normalized) == 0 {
			httpx.OK(w, map[string]any{
				"events":      []any{},
				"next_cursor": nil,
			})
			return
		}
		joined := strings.Join(normalized, ",")
		filters.Category = &joined
	}
	if c := q.Get("city"); c != "" {
		filters.City = &c
	}
	if df := q.Get("date_from"); df != "" {
		if t, err := time.Parse(time.RFC3339, df); err == nil {
			filters.DateFrom = &t
		}
	}
	if dt := q.Get("date_to"); dt != "" {
		if t, err := time.Parse(time.RFC3339, dt); err == nil {
			filters.DateTo = &t
		}
	}

	// Курсор: composite (новый) или legacy cursor_time.
	if cur := q.Get("cursor"); cur != "" {
		filters.CursorTime, filters.CursorID = decodeCursor(cur)
	} else if cur := q.Get("cursor_time"); cur != "" {
		if t, err := time.Parse(time.RFC3339, cur); err == nil {
			filters.CursorTime = &t
		}
	}

	if uid, ok := middleware.GetUserID(r.Context()); ok {
		filters.ViewerID = &uid
	}

	events, err := h.svc.GetArchive(r.Context(), filters)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	for i := range events {
		e := &events[i]
		if e.CanSeeExactLocation {
			continue
		}
		maskLatLng(e.ID, &e.Lat, &e.Lng)
		e.Address = nil
	}

	var nextCursor *string
	if len(events) > 0 {
		last := events[len(events)-1]
		c := encodeCursor(last.StartsAt, last.ID)
		nextCursor = &c
	}

	httpx.OK(w, map[string]any{
		"events":      events,
		"next_cursor": nextCursor,
	})
}

func (h *EventHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}

	var userIDPtr *uuid.UUID
	if uid, ok := middleware.GetUserID(r.Context()); ok {
		userIDPtr = &uid
	}
	role, _ := r.Context().Value(middleware.RoleKey).(string)

	event, err := h.svc.GetByID(r.Context(), id, userIDPtr)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "event not found")
		return
	}

	if event.Hidden {
		isOwner := userIDPtr != nil && event.OrganizerID == *userIDPtr
		isMod := role == "admin" || role == "moderator"
		if !isOwner && !isMod {
			httpx.Error(w, http.StatusNotFound, "not_found", "event not found")
			return
		}
	}

	canSee := event.CanSeeExactLocation

	lat := event.Lat
	lng := event.Lng
	if !canSee {
		maskLatLng(event.ID, &lat, &lng)
	}

	resp := map[string]any{
		"id":                      event.ID,
		"organizer_id":            event.OrganizerID,
		"organizer_name":          event.OrganizerName,
		"organizer_avatar":        event.OrganizerAvatar,
		"title":                   event.Title,
		"category":                event.Category,
		"description":             event.Description,
		"starts_at":               event.StartsAt,
		"city":                    event.City,
		"hide_exact_location":     event.HideExactLocation,
		"participant_limit":       event.ParticipantLimit,
		"participants_count":      event.ParticipantsCount,
		"user_application_status": event.UserApplicationStatus,
		"user_application_id":     event.UserApplicationID,
		"status":                  event.Status,
		"created_at":              event.CreatedAt,
		"can_see_exact_location":  canSee,
		"lat":                     lat,
		"lng":                     lng,
	}
	if canSee {
		resp["address"] = event.Address
	}

	httpx.OK(w, resp)
}

type createEventRequest struct {
	Title             string  `json:"title"`
	Category          string  `json:"category"`
	Description       string  `json:"description"`
	StartsAt          string  `json:"starts_at"`
	City              string  `json:"city"`
	Address           string  `json:"address"`
	Lat               float64 `json:"lat"`
	Lng               float64 `json:"lng"`
	HideExactLocation bool    `json:"hide_exact_location"`
	ParticipantLimit  *int32  `json:"participant_limit,omitempty"`
}

func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req createEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid starts_at, expected RFC3339")
		return
	}

	role, _ := r.Context().Value(middleware.RoleKey).(string)
	event, err := h.svc.Create(r.Context(), services.CreateEventInput{
		OrganizerID:       userID,
		Title:             req.Title,
		Category:          req.Category,
		Description:       req.Description,
		StartsAt:          startsAt,
		City:              req.City,
		Address:           req.Address,
		Lat:               req.Lat,
		Lng:               req.Lng,
		HideExactLocation: req.HideExactLocation,
		ParticipantLimit:  req.ParticipantLimit,
		Role:              role,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTooManyActiveEvents),
			errors.Is(err, services.ErrEventInPast),
			errors.Is(err, services.ErrInvalidParticipantLimit),
			errors.Is(err, services.ErrTitleRequired),
			errors.Is(err, services.ErrTitleTooLong),
			errors.Is(err, services.ErrDescriptionTooLong),
			errors.Is(err, services.ErrInvalidCategory),
			errors.Is(err, services.ErrInvalidLatitude),
			errors.Is(err, services.ErrInvalidLongitude),
			errors.Is(err, services.ErrCoordinatesMissing):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}

	httpx.JSON(w, http.StatusCreated, event)
}

type updateEventRequest struct {
	Title             string  `json:"title"`
	Category          string  `json:"category"`
	Description       string  `json:"description"`
	StartsAt          string  `json:"starts_at"`
	City              string  `json:"city"`
	Address           string  `json:"address"`
	Lat               float64 `json:"lat"`
	Lng               float64 `json:"lng"`
	HideExactLocation bool    `json:"hide_exact_location"`
	ParticipantLimit  *int32  `json:"participant_limit,omitempty"`
}

func (h *EventHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid event id")
		return
	}

	var req updateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid starts_at")
		return
	}

	event, err := h.svc.Update(r.Context(), services.UpdateEventInput{
		ID:                eventID,
		OrganizerID:       userID,
		Title:             req.Title,
		Category:          req.Category,
		Description:       req.Description,
		StartsAt:          startsAt,
		City:              req.City,
		Address:           req.Address,
		Lat:               req.Lat,
		Lng:               req.Lng,
		HideExactLocation: req.HideExactLocation,
		ParticipantLimit:  req.ParticipantLimit,
	})

	if err != nil {
		switch {
		case errors.Is(err, services.ErrTooLateToEdit),
			errors.Is(err, services.ErrCannotEditInactive),
			errors.Is(err, services.ErrNotEventOwner):
			httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		case errors.Is(err, services.ErrEventInPast),
			errors.Is(err, services.ErrParticipantLimitTooLow),
			errors.Is(err, services.ErrInvalidParticipantLimit),
			errors.Is(err, services.ErrTitleRequired),
			errors.Is(err, services.ErrTitleTooLong),
			errors.Is(err, services.ErrDescriptionTooLong),
			errors.Is(err, services.ErrInvalidCategory),
			errors.Is(err, services.ErrInvalidLatitude),
			errors.Is(err, services.ErrInvalidLongitude),
			errors.Is(err, services.ErrCoordinatesMissing):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}

	httpx.OK(w, event)
}

type cancelEventRequest struct {
	Reason string `json:"reason"`
}

func (h *EventHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}

	var req cancelEventRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	event, err := h.svc.Cancel(r.Context(), eventID, userID, req.Reason)
	if err != nil {
		if errors.Is(err, services.ErrNotEventOwner) {
			httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		if errors.Is(err, services.ErrEventNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	httpx.OK(w, event)
}

func (h *EventHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	events, err := h.svc.ListMyOrganized(r.Context(), userID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	httpx.OK(w, map[string]any{"events": events})
}

// maskLatLng применяет детерминированный сдвиг до ~500 м к координатам.
func maskLatLng(eventID uuid.UUID, lat, lng *pgtype.Float8) {
	if !lat.Valid || !lng.Valid {
		return
	}
	sum := sha256.Sum256(eventID[:])
	ux := binary.BigEndian.Uint32(sum[0:4])
	uy := binary.BigEndian.Uint32(sum[4:8])
	fx := float64(ux) / float64(1<<32)
	fy := float64(uy) / float64(1<<32)
	dx := fx*2 - 1
	dy := fy*2 - 1

	const shiftDeg = 0.0045

	lat.Float64 += dy * shiftDeg
	cosLat := math.Cos(lat.Float64 * math.Pi / 180)
	if cosLat < 0.1 {
		cosLat = 0.1
	}
	lng.Float64 += dx * shiftDeg / cosLat
}

// encodeCursor — композитный курсор `(starts_at, id)` в одну строку.
// Формат: <RFC3339Nano>|<uuid>. Читаемо в логах, легко парсить.
func encodeCursor(startsAt time.Time, id uuid.UUID) string {
	return startsAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()
}

// decodeCursor — обратный к encodeCursor.
// Для обратной совместимости, если в строке нет `|`, трактуем её как
// только starts_at (старый клиент): id будет nil, tuple comparison
// выродится в `> starts_at` — то же поведение, что было.
func decodeCursor(s string) (startsAt *time.Time, id *uuid.UUID) {
	if s == "" {
		return nil, nil
	}
	if i := strings.Index(s, "|"); i >= 0 {
		t, err := time.Parse(time.RFC3339Nano, s[:i])
		if err != nil {
			return nil, nil
		}
		parsedID, err := uuid.Parse(s[i+1:])
		if err != nil {
			return &t, nil
		}
		return &t, &parsedID
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, nil
	}
	return &t, nil
}

// participantView — то, что видит клиент в списке участников.
// Приватные поля (email, telegram, vk, gender, bio) — НЕ включаем.
type participantView struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	AvatarURL   *string   `json:"avatar_url"`
	City        *string   `json:"city"`
	Age         *int      `json:"age,omitempty"`
	IsOrganizer bool      `json:"is_organizer"`
}

// ListParticipants — публичный эндпоинт: GET /events/{id}/participants.
// Возвращает только approved (орг первым). Возраст — только если show_age.
// Гость видит то же, что залогиненный (approved публичны).
func (h *EventHandler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid event id")
		return
	}

	rows, err := h.svc.ListParticipants(r.Context(), eventID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	// Если события нет — пустой список (не 404: это не «страница события»,
	// а сайдбар; на самой странице события 404 отдаст GetByID).
	participants := make([]participantView, 0, len(rows))
	for _, row := range rows {
		pv := participantView{
			ID:          row.ID,
			Username:    row.Username,
			AvatarURL:   row.AvatarUrl,
			City:        row.City,
			IsOrganizer: row.IsOrganizer,
		}
		if row.ShowAge && row.BirthDate.Valid {
			age := calculateAge(row.BirthDate.Time)
			pv.Age = &age
		}
		participants = append(participants, pv)
	}

	httpx.OK(w, map[string]any{"participants": participants})
}
