package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/SaratovDev/Namylimsya_App/api/internal/httpx"
	"github.com/SaratovDev/Namylimsya_App/api/internal/middleware"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
)

type SupportHandler struct {
	svc *services.SupportService
}

func NewSupportHandler(svc *services.SupportService) *SupportHandler {
	return &SupportHandler{svc: svc}
}

// ─── Tickets ──────────────────────────────────────────────────────────

type createTicketRequest struct {
	Type    string `json:"type"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

func (h *SupportHandler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	var req createTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	var userIDPtr *uuid.UUID
	if uid, ok := middleware.GetUserID(r.Context()); ok {
		userIDPtr = &uid
	}

	t, err := h.svc.CreateTicket(r.Context(), userIDPtr, req.Type, req.Subject, req.Message)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidTicketType),
			errors.Is(err, services.ErrSubjectTooLong),
			errors.Is(err, services.ErrMessageEmpty),
			errors.Is(err, services.ErrMessageTooLong):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
}

func (h *SupportHandler) ListTickets(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	limit := int32(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = int32(v)
		}
	}
	offset := int32(0)
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}

	tickets, err := h.svc.ListTickets(r.Context(), statusPtr, limit, offset)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, map[string]any{"tickets": tickets})
}

func (h *SupportHandler) ResolveTicket(w http.ResponseWriter, r *http.Request) {
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
	var req ticketActionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	t, err := h.svc.ResolveTicket(r.Context(), adminID, id, req.Comment)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTicketNotFound):
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, services.ErrTicketClosed):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}
	httpx.OK(w, t)
}

// ─── Complaints ───────────────────────────────────────────────────────

type createComplaintRequest struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Reason     string `json:"reason"`
}

func (h *SupportHandler) CreateComplaint(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req createComplaintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	targetID, err := uuid.Parse(req.TargetID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid target_id")
		return
	}

	c, err := h.svc.CreateComplaint(r.Context(), userID, req.TargetType, targetID, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidTargetType),
			errors.Is(err, services.ErrReasonEmpty),
			errors.Is(err, services.ErrReasonTooLong):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *SupportHandler) ListComplaints(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	limit := int32(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = int32(v)
		}
	}
	offset := int32(0)
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = int32(v)
		}
	}

	complaints, err := h.svc.ListComplaints(r.Context(), statusPtr, limit, offset)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, map[string]any{"complaints": complaints})
}

func (h *SupportHandler) GetComplaint(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	d, err := h.svc.GetComplaint(r.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrComplaintNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, d)
}

type resolveComplaintRequest struct {
	Comment string `json:"comment"`
}

func (h *SupportHandler) ResolveComplaint(w http.ResponseWriter, r *http.Request) {
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
	var req resolveComplaintRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	c, err := h.svc.ResolveComplaint(r.Context(), adminID, id, req.Comment)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrComplaintNotFound):
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, services.ErrComplaintClosed):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}
	httpx.OK(w, c)
}

func (h *SupportHandler) RejectComplaint(w http.ResponseWriter, r *http.Request) {
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
	var req resolveComplaintRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	c, err := h.svc.RejectComplaint(r.Context(), adminID, id, req.Comment)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrComplaintNotFound):
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, services.ErrComplaintClosed):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}
	httpx.OK(w, c)
}

// ─── Messages ─────────────────────────────────────────────────────────

func (h *SupportHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.DeleteMessage(r.Context(), adminID, id); err != nil {
		if errors.Is(err, services.ErrMessageNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		httpx.InternalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ticketActionRequest struct {
	Comment string `json:"comment"`
}

func (h *SupportHandler) GetTicket(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid id")
		return
	}
	d, err := h.svc.GetTicket(r.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrTicketNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, d)
}

func (h *SupportHandler) RejectTicket(w http.ResponseWriter, r *http.Request) {
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
	var req ticketActionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	t, err := h.svc.RejectTicket(r.Context(), adminID, id, req.Comment)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTicketNotFound):
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, services.ErrTicketClosed):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}
	httpx.OK(w, t)
}

type replyTicketRequest struct {
	Body string `json:"body"`
}

func (h *SupportHandler) ReplyToTicket(w http.ResponseWriter, r *http.Request) {
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
	var req replyTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	reply, err := h.svc.ReplyToTicket(r.Context(), adminID, id, req.Body)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTicketNotFound):
			httpx.Error(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, services.ErrReplyBodyEmpty),
			errors.Is(err, services.ErrReplyBodyTooLong):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}
	httpx.JSON(w, http.StatusCreated, reply)
}
