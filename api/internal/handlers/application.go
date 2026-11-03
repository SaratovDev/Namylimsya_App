package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/SaratovDev/Namylimsya_App/api/internal/httpx"
	"github.com/SaratovDev/Namylimsya_App/api/internal/middleware"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ApplicationHandler struct {
	svc *services.ApplicationService
}

func NewApplicationHandler(svc *services.ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{svc: svc}
}

func (h *ApplicationHandler) Apply(w http.ResponseWriter, r *http.Request) {
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

	app, err := h.svc.Apply(r.Context(), eventID, userID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	httpx.JSON(w, http.StatusCreated, app)
}

func (h *ApplicationHandler) Approve(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid application id")
		return
	}

	app, err := h.svc.Approve(r.Context(), appID, userID)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	httpx.OK(w, app)
}

func (h *ApplicationHandler) Reject(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid application id")
		return
	}

	app, err := h.svc.Reject(r.Context(), appID, userID)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	httpx.OK(w, app)
}

type cancelApplicationRequest struct {
	Reason string `json:"reason"`
}

func (h *ApplicationHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	appID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid application id")
		return
	}

	var req cancelApplicationRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	app, err := h.svc.Cancel(r.Context(), appID, userID, req.Reason)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	httpx.OK(w, app)
}

func (h *ApplicationHandler) ListForEvent(w http.ResponseWriter, r *http.Request) {
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

	apps, err := h.svc.ListForEvent(r.Context(), eventID, userID)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	httpx.OK(w, map[string]any{"applications": apps})
}
