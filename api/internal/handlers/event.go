package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/httpx"
	"github.com/SaratovDev/Namylimsya_App/api/internal/middleware"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type EventHandler struct {
	svc *services.EventService
}

func NewEventHandler(svc *services.EventService) *EventHandler {
	return &EventHandler{svc: svc}
}

type createEventRequest struct {
	Title             string  `json:"title"`
	Category          string  `json:"category"`
	Description       string  `json:"description"`
	StartsAt          string  `json:"starts_at"` // RFC3339
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
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	httpx.JSON(w, http.StatusCreated, event)
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

	if c := q.Get("category"); c != "" {
		filters.Category = &c
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
	if cur := q.Get("cursor_time"); cur != "" {
		if t, err := time.Parse(time.RFC3339, cur); err == nil {
			filters.CursorTime = &t
		}
	}

	events, err := h.svc.GetFeed(r.Context(), filters)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	// Формируем next_cursor для клиента
	var nextCursor *string
	if len(events) > 0 {
		lastTime := events[len(events)-1].StartsAt.Format(time.RFC3339)
		nextCursor = &lastTime
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

	event, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "event not found")
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
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	httpx.OK(w, event)
}
