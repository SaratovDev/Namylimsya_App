package handlers

import (
	"net/http"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/ws"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type WSNotificationHandler struct {
	hub      *ws.NotificationHub
	tm       *auth.TokenManager
	sm       *auth.SessionManager
	upgrader websocket.Upgrader
}

func NewWSNotificationHandler(hub *ws.NotificationHub, tm *auth.TokenManager, sm *auth.SessionManager, allowedOrigin string) *WSNotificationHandler {
	return &WSNotificationHandler{
		hub: hub, tm: tm, sm: sm,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			Subprotocols:    []string{wsSubprotocolBearer},
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				return origin == allowedOrigin
			},
		},
	}
}

func (h *WSNotificationHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// См. комментарий в ws.go — отклоняем legacy-клиентов (без subprotocol)
	// ДО Upgrade, иначе gorilla пропустит handshake.
	if !hasBearerSubprotocol(r) {
		http.Error(w, "Sec-WebSocket-Protocol 'bearer' is required", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	token := extractBearerFromSubprotocol(r)
	if token == "" {
		closeWithCode(conn, wsCloseUnauthorized, "missing token")
		return
	}

	claims, err := h.tm.ParseAccessToken(token)
	if err != nil {
		closeWithCode(conn, wsCloseUnauthorized, "invalid token")
		return
	}
	if h.sm != nil && h.sm.IsBanned(r.Context(), claims.UserID) {
		closeWithCode(conn, wsCloseForbidden, "banned")
		return
	}

	client := ws.NewNotificationClient(h.hub, conn, claims.UserID)
	h.hub.Register(client)
	defer h.hub.Unregister(client)

	conn.SetReadLimit(wsMaxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	// Писатель
	go func() {
		ticker := time.NewTicker(wsPingPeriod)
		defer func() {
			ticker.Stop()
			_ = conn.Close()
		}()
		for {
			select {
			case data, ok := <-client.Send():
				_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if !ok {
					code := client.CloseCode()
					if code == 0 {
						code = websocket.CloseNormalClosure
					}
					_ = conn.WriteMessage(websocket.CloseMessage,
						websocket.FormatCloseMessage(code, ""))
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					return
				}
			case <-ticker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// Читатель — клиент ничего не присылает, но нужно ловить разрыв.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			if isNormalWSClose(err) {
				log.Debug().Err(err).
					Str("user_id", claims.UserID.String()).
					Msg("ws notification closed")
			} else {
				log.Warn().Err(err).
					Str("user_id", claims.UserID.String()).
					Msg("ws notification read error")
			}
			break
		}
	}
}
