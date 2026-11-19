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
	"github.com/SaratovDev/Namylimsya_App/api/internal/ws"
)

type ChatHandler struct {
	svc *services.ChatService
	hub *ws.Hub
}

func NewChatHandler(svc *services.ChatService, hub *ws.Hub) *ChatHandler {
	return &ChatHandler{svc: svc, hub: hub}
}

// mapChatErr — единая маппинг ошибок чата:
//   - ErrNotChatMember → 403 без деталей (не палим текст ошибки БД);
//   - ErrMessageEmpty / ErrMessageTooLong → 400 с текстом (это про ввод);
//   - всё остальное (БД упала, событие не найдено и т.п.) → 500 generic.
//
// Раньше любая ошибка GetHistory/Send отдавалась как 403 с err.Error(),
// и юзер видел SQL-ошибку при недоступной БД.
func mapChatErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrNotChatMember):
		httpx.Error(w, http.StatusForbidden, "forbidden", "you are not a member of this chat")
	case errors.Is(err, services.ErrMessageEmpty),
		errors.Is(err, services.ErrMessageTooLong):
		httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
	default:
		httpx.InternalError(w, err)
	}
}

func (h *ChatHandler) History(w http.ResponseWriter, r *http.Request) {
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

	limit := int32(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = int32(v)
		}
	}
	var cursor *time.Time
	if c := r.URL.Query().Get("cursor_time"); c != "" {
		if t, err := time.Parse(time.RFC3339, c); err == nil {
			cursor = &t
		}
	}

	messages, err := h.svc.GetHistory(r.Context(), eventID, userID, cursor, limit)
	if err != nil {
		mapChatErr(w, err)
		return
	}

	_ = h.svc.MarkRead(r.Context(), eventID, userID)

	httpx.OK(w, map[string]any{"messages": messages})
}

type sendMessageRequest struct {
	Text      string     `json:"text"`
	ReplyToID *uuid.UUID `json:"reply_to_id,omitempty"`
}

func (h *ChatHandler) SendHTTP(w http.ResponseWriter, r *http.Request) {
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

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	msg, err := h.svc.Send(r.Context(), services.SendMessageInput{
		EventID: eventID, UserID: userID, Text: req.Text, ReplyToID: req.ReplyToID,
	})
	if err != nil {
		mapChatErr(w, err)
		return
	}

	if h.hub != nil {
		user, _ := h.svc.GetUser(r.Context(), userID)
		userName := "Пользователь"
		var userAvatar *string
		if user != nil {
			userName = user.Username
			userAvatar = user.AvatarUrl
		}
		h.hub.Broadcast(eventID, map[string]any{
			"type": "message",
			"message": map[string]any{
				"id":          msg.ID,
				"event_id":    msg.EventID,
				"user_id":     msg.UserID,
				"text":        msg.Text,
				"reply_to_id": msg.ReplyToID,
				"created_at":  msg.CreatedAt,
				"user_name":   userName,
				"user_avatar": userAvatar,
			},
		})
	}

	httpx.JSON(w, http.StatusCreated, msg)
}

func (h *ChatHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
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
	_ = h.svc.MarkRead(r.Context(), eventID, userID)
	w.WriteHeader(http.StatusNoContent)
}
