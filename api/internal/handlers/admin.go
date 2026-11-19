package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/SaratovDev/Namylimsya_App/api/internal/httpx"
	"github.com/SaratovDev/Namylimsya_App/api/internal/middleware"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
)

type AdminHandler struct {
	svc *services.AdminService
}

func NewAdminHandler(svc *services.AdminService) *AdminHandler {
	return &AdminHandler{svc: svc}
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.GetDashboard(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, d)
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var search, role *string
	if s := q.Get("search"); s != "" {
		search = &s
	}
	if ro := q.Get("role"); ro != "" {
		role = &ro
	}
	limit := int32(50)
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = int32(v)
		}
	}
	offset := int32(0)
	if o := q.Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}

	users, err := h.svc.ListUsers(r.Context(), search, role, limit, offset)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, map[string]any{"users": users})
}

func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}

	details, err := h.svc.GetUserDetails(r.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrAdminUserNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, details)
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

func (h *AdminHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	u, err := h.svc.UpdateUserRole(r.Context(), adminID, id, req.Role)
	if err != nil {
		if errors.Is(err, services.ErrInvalidRole) {
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		if errors.Is(err, services.ErrAdminUserNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, u)
}

type banRequest struct {
	Banned    bool    `json:"banned"`
	Reason    string  `json:"reason"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

func (h *AdminHandler) BanUser(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	var req banRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "bad_request",
				"invalid expires_at, expected RFC3339")
			return
		}
		expiresAt = &t
	}

	u, err := h.svc.BanUser(r.Context(), adminID, id, req.Banned, req.Reason, expiresAt)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrBanReasonRequired),
			errors.Is(err, services.ErrBanExpiresInPast),
			errors.Is(err, services.ErrCannotBanSelf),
			errors.Is(err, services.ErrModeratorCannotBanAdmin):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		case errors.Is(err, services.ErrAdminUserNotFound):
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}
	httpx.OK(w, u)
}

func (h *AdminHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var status, search, category, city *string
	if s := q.Get("status"); s != "" {
		status = &s
	}
	if s := q.Get("search"); s != "" {
		search = &s
	}
	if s := q.Get("category"); s != "" {
		category = &s
	}
	if s := q.Get("city"); s != "" {
		city = &s
	}

	limit := int32(50)
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = int32(v)
		}
	}
	offset := int32(0)
	if o := q.Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}

	events, err := h.svc.ListEvents(r.Context(), status, search, category, city, limit, offset)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, map[string]any{"events": events})
}

func (h *AdminHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if err := h.svc.DeleteEvent(r.Context(), adminID, id); err != nil {
		httpx.InternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}

	details, err := h.svc.GetEventDetails(r.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrEventNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, details)
}

type adminCancelEventRequest struct {
	Reason string `json:"reason"`
}

func (h *AdminHandler) CancelEvent(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}

	var req adminCancelEventRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	event, err := h.svc.CancelEvent(r.Context(), adminID, id, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrEventNotFound):
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, services.ErrEventNotActive),
			errors.Is(err, services.ErrEventAlreadyCancelled):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}
	httpx.OK(w, event)
}

func (h *AdminHandler) HideEvent(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if err := h.svc.HideEvent(r.Context(), adminID, id); err != nil {
		if errors.Is(err, services.ErrEventNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, map[string]any{"hidden": true})
}

func (h *AdminHandler) UnhideEvent(w http.ResponseWriter, r *http.Request) {
	adminID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	if err := h.svc.UnhideEvent(r.Context(), adminID, id); err != nil {
		if errors.Is(err, services.ErrEventNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, map[string]any{"hidden": false})
}

func (h *AdminHandler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var adminID *uuid.UUID
	if s := q.Get("admin_id"); s != "" {
		parsed, err := uuid.Parse(s)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid admin_id")
			return
		}
		adminID = &parsed
	}

	var action, targetType *string
	if a := q.Get("action"); a != "" {
		action = &a
	}
	if tt := q.Get("target_type"); tt != "" {
		targetType = &tt
	}

	limit := int32(50)
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 200 {
			limit = int32(v)
		}
	}
	offset := int32(0)
	if o := q.Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}

	entries, err := h.svc.ListAuditLog(r.Context(), adminID, action, targetType, limit, offset)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}

	httpx.OK(w, map[string]any{"entries": entries})
}
