package handlers

import (
	"net/http"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/ws"
	"github.com/gorilla/websocket"
)

type WSNotificationHandler struct {
	hub *ws.NotificationHub
	tm  *auth.TokenManager
}

func NewWSNotificationHandler(hub *ws.NotificationHub, tm *auth.TokenManager) *WSNotificationHandler {
	return &WSNotificationHandler{hub: hub, tm: tm}
}

var notifUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (h *WSNotificationHandler) Handle(w http.ResponseWriter, r *http.Request) {
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

	conn, err := notifUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := ws.NewNotificationClient(h.hub, conn, claims.UserID)
	h.hub.Register(client)
	defer func() {
		h.hub.Unregister(client)
		conn.Close()
	}()

	// Писатель
	go func() {
		for data := range client.Send() {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}()

	// Читатель — нам не нужны сообщения от клиента, но нужно ловить разрыв
	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
