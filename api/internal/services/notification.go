package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/email"
	"github.com/google/uuid"
)

type NotificationService struct {
	db     *dbgen.Queries
	mailer *email.Sender

	notifyWS     func(userID uuid.UUID, payload map[string]any)
	enqueueEmail func(userID uuid.UUID, userEmail, notifType string, payload map[string]any)
}

func NewNotificationService(db *dbgen.Queries, mailer *email.Sender) *NotificationService {
	return &NotificationService{db: db, mailer: mailer}
}

func (s *NotificationService) SetWSNotifier(fn func(userID uuid.UUID, payload map[string]any)) {
	s.notifyWS = fn
}

func (s *NotificationService) SetEmailEnqueuer(fn func(userID uuid.UUID, userEmail, notifType string, payload map[string]any)) {
	s.enqueueEmail = fn
}

type CreateInput struct {
	UserID    uuid.UUID
	Type      string
	Payload   map[string]any
	SendEmail bool
}

func (s *NotificationService) Create(ctx context.Context, input CreateInput) (*dbgen.Notification, error) {
	payloadJSON, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	rawMsg := json.RawMessage(payloadJSON)

	n, err := s.db.CreateNotification(ctx, dbgen.CreateNotificationParams{
		UserID:  input.UserID,
		Type:    input.Type,
		Payload: &rawMsg,
	})
	if err != nil {
		return nil, err
	}

	// Реалтайм через WS
	if s.notifyWS != nil {
		s.notifyWS(input.UserID, map[string]any{
			"id":         n.ID,
			"type":       n.Type,
			"payload":    input.Payload,
			"is_read":    false,
			"created_at": n.CreatedAt,
		})
	}

	// Email — в очередь на батчинг
	if input.SendEmail && s.enqueueEmail != nil {
		user, err := s.db.GetUserByID(ctx, input.UserID)
		if err == nil && user.Email != "" {
			s.enqueueEmail(input.UserID, user.Email, input.Type, input.Payload)
		}
	}

	return &n, nil
}

func (s *NotificationService) List(ctx context.Context, userID uuid.UUID, cursorTime *time.Time, limit int32) ([]dbgen.Notification, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.db.GetNotifications(ctx, dbgen.GetNotificationsParams{
		UserID:     userID,
		CursorTime: cursorTime,
		Limit:      limit,
	})
}

func (s *NotificationService) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.db.CountUnreadNotifications(ctx, userID)
}

func (s *NotificationService) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	return s.db.MarkNotificationRead(ctx, dbgen.MarkNotificationReadParams{ID: id, UserID: userID})
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.db.MarkAllNotificationsRead(ctx, userID)
}
