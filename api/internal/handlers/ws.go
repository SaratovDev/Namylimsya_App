package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/ratelimit"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
	"github.com/SaratovDev/Namylimsya_App/api/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// В dev разрешаем всё; в prod — ограничить
		return true
	},
}

type WSHandler struct {
	hub     *ws.Hub
	chatSvc *services.ChatService
	tm      *auth.TokenManager
	rl      *ratelimit.Limiter
}

func NewWSHandler(hub *ws.Hub, chatSvc *services.ChatService, tm *auth.TokenManager, rl *ratelimit.Limiter) *WSHandler {
	return &WSHandler{hub: hub, chatSvc: chatSvc, tm: tm, rl: rl}
}

type incomingMessage struct {
	Text      string     `json:"text"`
	ReplyToID *uuid.UUID `json:"reply_to_id,omitempty"`
}

func (h *WSHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// 1. Токен из query (браузерный WebSocket не поддерживает Authorization)
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	claims, err := h.tm.ParseAccessToken(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// 2. event_id из URL
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}

	// 3. Проверка доступа
	canAccess, err := h.chatSvc.CanAccessChat(r.Context(), eventID, claims.UserID)
	if err != nil || !canAccess {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// 4. Upgrade
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := ws.NewClient(h.hub, conn, eventID, claims.UserID)
	h.hub.Register(client)
	defer func() {
		h.hub.Unregister(client)
		conn.Close()
	}()

	// 5. Писатель: отправляет всё из send в WS
	go func() {
		for data := range client.Send() {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}()

	// 6. Читатель: принимает сообщения, сохраняет, бродкастит
	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// rate limit: 5 сообщений / 10 сек
		rlKey := "chat:" + claims.UserID.String()
		allowed, _ := h.rl.Allow(r.Context(), rlKey, 5, 10*time.Second)
		if !allowed {
			errMsg, _ := json.Marshal(map[string]string{"error": "rate limit exceeded"})
			client.Enqueue(errMsg)
			continue
		}

		var msg incomingMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		saved, err := h.chatSvc.Send(ctx, services.SendMessageInput{
			EventID:   eventID,
			UserID:    claims.UserID,
			Text:      msg.Text,
			ReplyToID: msg.ReplyToID,
		})
		cancel()
		if err != nil {
			errMsg, _ := json.Marshal(map[string]string{"error": err.Error()})
			client.Enqueue(errMsg)
			continue
		}

		// BroadCast
		out, _ := json.Marshal(map[string]any{
			"type": "message",
			"message": map[string]any{
				"id":          saved.ID,
				"event_id":    saved.EventID,
				"user_id":     saved.UserID,
				"text":        saved.Text,
				"reply_to_id": saved.ReplyToID,
				"created_at":  saved.CreatedAt,
			},
		})
		h.hub.Broadcast(eventID, json.RawMessage(out))
	}
}
