package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/ratelimit"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
	"github.com/SaratovDev/Namylimsya_App/api/internal/ws"
)

const (
	wsWriteWait      = 10 * time.Second
	wsPongWait       = 60 * time.Second
	wsPingPeriod     = (wsPongWait * 9) / 10
	wsMaxMessageSize = 8192

	// wsSubprotocolBearer — маркер в Sec-WebSocket-Protocol.
	// Клиент: new WebSocket(url, ["bearer", "<jwt>"]).
	// Сервер отвечает "bearer", и мы знаем, что handshake согласован.
	wsSubprotocolBearer = "bearer"
)

// Application close-коды (видит клиент через onclose):
//   - 4400 — плохой запрос (невалидный event_id);
//   - 4401 — нет/протух access-токен → клиент делает refresh;
//   - 4403 — нет доступа к чату / забанен → denied;
//   - 4408 — соединение заменено более новым (LRU per-user cap) → не reconnect.
const (
	wsCloseBadRequest   = 4400
	wsCloseUnauthorized = 4401
	wsCloseForbidden    = 4403
)

type WSHandler struct {
	hub      *ws.Hub
	chatSvc  *services.ChatService
	tm       *auth.TokenManager
	sm       *auth.SessionManager
	rl       *ratelimit.Limiter
	upgrader websocket.Upgrader
}

// NewWSHandler — allowedOrigin сравнивается с Origin (WebSocket CheckOrigin).
// Подпротокол "bearer" обязателен: без него Upgrade вернёт 400.
func NewWSHandler(hub *ws.Hub, chatSvc *services.ChatService, tm *auth.TokenManager, sm *auth.SessionManager, rl *ratelimit.Limiter, allowedOrigin string) *WSHandler {
	return &WSHandler{
		hub: hub, chatSvc: chatSvc, tm: tm, sm: sm, rl: rl,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			Subprotocols:    []string{wsSubprotocolBearer},
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				// Пустой Origin — не браузер (curl, нативные клиенты). Пропускаем:
				// без валидного токена всё равно не будет.
				if origin == "" {
					return true
				}
				return origin == allowedOrigin
			},
		},
	}
}

// hasBearerSubprotocol — true, если клиент передал "bearer" среди
// Sec-WebSocket-Protocol. Проверяем ДО Upgrade — иначе gorilla молча
// согласует "" и пропустит handshake без subprotocol.
func hasBearerSubprotocol(r *http.Request) bool {
	for _, v := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, part := range strings.Split(v, ",") {
			if strings.TrimSpace(part) == wsSubprotocolBearer {
				return true
			}
		}
	}
	return false
}

// extractBearerFromSubprotocol достаёт JWT из Sec-WebSocket-Protocol.
// Клиент передаёт ["bearer", "<jwt>"]; заголовок приходит одной строкой
// "bearer, <jwt>". Возвращает "" если токена нет.
func extractBearerFromSubprotocol(r *http.Request) string {
	for _, v := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, part := range strings.Split(v, ",") {
			p := strings.TrimSpace(part)
			if p == "" || p == wsSubprotocolBearer {
				continue
			}
			return p
		}
	}
	return ""
}

// closeWithCode отправляет WS Close-frame с прикладным кодом и коротким
// reason, чтобы клиент получил onclose с этим кодом, а не 1006.
func closeWithCode(conn *websocket.Conn, code int, reason string) {
	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	_ = conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(code, reason),
		time.Now().Add(wsWriteWait),
	)
}

type incomingMessage struct {
	Text      string     `json:"text"`
	ReplyToID *uuid.UUID `json:"reply_to_id,omitempty"`
}

func (h *WSHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Сначала — handshake. Ошибки после upgrade отправляем через close-код.
	// Если клиент не передал subprotocol "bearer", Upgrade сам вернёт 400.
	if !hasBearerSubprotocol(r) {
		http.Error(w, "Sec-WebSocket-Protocol 'bearer' is required", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 1. Токен из subprotocol.
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

	// 2. Бан
	if h.sm != nil && h.sm.IsBanned(r.Context(), claims.UserID) {
		closeWithCode(conn, wsCloseForbidden, "banned")
		return
	}

	// 3. event_id
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		closeWithCode(conn, wsCloseBadRequest, "invalid event id")
		return
	}

	// 4. Доступ
	canAccess, err := h.chatSvc.CanAccessChat(r.Context(), eventID, claims.UserID)
	if err != nil || !canAccess {
		closeWithCode(conn, wsCloseForbidden, "forbidden")
		return
	}

	client := ws.NewClient(h.hub, conn, eventID, claims.UserID)
	// Register может LRU-эвиктить старейшее соединение этого юзера (4408).
	h.hub.Register(client)
	defer h.hub.Unregister(client)

	// 5. Keepalive
	conn.SetReadLimit(wsMaxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	// 6. Писатель: данные из send или тикер пингов. При закрытии канала
	// send — отправляем CloseMessage с кодом из client.CloseCode() (4408
	// при LRU-эвикции, 1008 при перегрузке канала, 1000 при EOF).
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

	// 7. Читатель
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			// Нормальные close-коды (клиент ушёл, вкладка закрылась,
			// LRU-эвикция 4408, close без статуса 1005) — не WRN, а Debug.
			// WRN оставляем только для аномалий (1006 abnormal, 1011 internal,
			// timeout, network error).
			if isNormalWSClose(err) {
				log.Debug().Err(err).
					Str("event_id", eventID.String()).
					Str("user_id", claims.UserID.String()).
					Msg("ws chat closed")
			} else {
				log.Warn().Err(err).
					Str("event_id", eventID.String()).
					Str("user_id", claims.UserID.String()).
					Msg("ws chat read error")
			}
			break
		}
		log.Debug().
			Str("event_id", eventID.String()).
			Str("user_id", claims.UserID.String()).
			Int("bytes", len(data)).
			Msg("ws chat recv")

		// rate limit: 5 сообщений / 10 сек.
		rlCtx, rlCancel := context.WithTimeout(context.Background(), 2*time.Second)
		rlKey := "chat:" + claims.UserID.String()
		allowed, _ := h.rl.Allow(rlCtx, rlKey, 5, 10*time.Second)
		rlCancel()
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
		if err != nil {
			cancel()
			log.Warn().Err(err).
				Str("event_id", eventID.String()).
				Str("user_id", claims.UserID.String()).
				Msg("ws chat send failed")
			errMsg, _ := json.Marshal(map[string]string{"error": err.Error()})
			client.Enqueue(errMsg)
			continue
		}
		log.Debug().
			Str("event_id", eventID.String()).
			Str("user_id", claims.UserID.String()).
			Str("message_id", saved.ID.String()).
			Msg("ws chat broadcast")

		user, userErr := h.chatSvc.GetUser(ctx, claims.UserID)
		cancel()

		userName := "Пользователь"
		var userAvatar *string
		if userErr == nil {
			userName = user.Username
			userAvatar = user.AvatarUrl
		}

		out, _ := json.Marshal(map[string]any{
			"type": "message",
			"message": map[string]any{
				"id":          saved.ID,
				"event_id":    saved.EventID,
				"user_id":     saved.UserID,
				"text":        saved.Text,
				"reply_to_id": saved.ReplyToID,
				"created_at":  saved.CreatedAt,
				"user_name":   userName,
				"user_avatar": userAvatar,
			},
		})
		h.hub.Broadcast(eventID, out)
	}
}

// isNormalWSClose — true для close-кодов, которые не являются аномалией:
// клиент штатно ушёл (1000 normal, 1001 going away), нет статуса (1005),
// LRU-эвикция (4408), app-level ban/forbidden (4401/4403) — это уже
// обработано на handshake, но close-frame может прийти позже.
// Аномалии (1006 abnormal, 1011 internal, network timeout) — остаются WRN.
func isNormalWSClose(err error) bool {
	if err == nil {
		return false
	}
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		switch closeErr.Code {
		case websocket.CloseNormalClosure, // 1000
			websocket.CloseGoingAway,        // 1001
			websocket.CloseNoStatusReceived, // 1005
			4401,                            // unauthorized (close после upgrade)
			4403,                            // forbidden
			4408:                            // replaced by newer connection (LRU)
			return true
		}
	}
	return false
}
