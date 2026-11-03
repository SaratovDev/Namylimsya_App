package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/google/uuid"
)

var (
	ErrNotChatMember  = errors.New("you are not a member of this chat")
	ErrMessageTooLong = errors.New("message is too long (max 1000 chars)")
	ErrMessageEmpty   = errors.New("message cannot be empty")
)

type ChatService struct {
	db *dbgen.Queries
}

func NewChatService(db *dbgen.Queries) *ChatService {
	return &ChatService{db: db}
}

func (s *ChatService) CanAccessChat(ctx context.Context, eventID, userID uuid.UUID) (bool, error) {
	event, err := s.db.GetEventByID(ctx, eventID)
	if err != nil {
		return false, err
	}
	if event.OrganizerID == userID {
		return true, nil
	}
	app, err := s.db.GetUserApplicationForEvent(ctx, dbgen.GetUserApplicationForEventParams{
		EventID: eventID, UserID: userID,
	})
	if err != nil {
		return false, nil
	}
	return app.Status == "approved", nil
}

type SendMessageInput struct {
	EventID   uuid.UUID
	UserID    uuid.UUID
	Text      string
	ReplyToID *uuid.UUID
}

func (s *ChatService) Send(ctx context.Context, input SendMessageInput) (*dbgen.ChatMessage, error) {
	text := strings.TrimSpace(input.Text)
	if text == "" {
		return nil, ErrMessageEmpty
	}
	if len([]rune(text)) > 1000 {
		return nil, ErrMessageTooLong
	}

	canAccess, err := s.CanAccessChat(ctx, input.EventID, input.UserID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, ErrNotChatMember
	}

	msg, err := s.db.CreateMessage(ctx, dbgen.CreateMessageParams{
		EventID:   input.EventID,
		UserID:    input.UserID,
		Text:      text,
		ReplyToID: input.ReplyToID, // *uuid.UUID напрямую
	})
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

type MessageWithUser struct {
	ID         uuid.UUID  `json:"id"`
	EventID    uuid.UUID  `json:"event_id"`
	UserID     uuid.UUID  `json:"user_id"`
	Text       string     `json:"text"`
	ReplyToID  *uuid.UUID `json:"reply_to_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UserName   string     `json:"user_name"`
	UserAvatar *string    `json:"user_avatar,omitempty"`
}

func (s *ChatService) GetHistory(ctx context.Context, eventID, userID uuid.UUID, cursorTime *time.Time, limit int32) ([]MessageWithUser, error) {
	canAccess, err := s.CanAccessChat(ctx, eventID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, ErrNotChatMember
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows, err := s.db.GetMessages(ctx, dbgen.GetMessagesParams{
		EventID:    eventID,
		Limit:      limit,
		CursorTime: cursorTime, // *time.Time напрямую
	})
	if err != nil {
		return nil, err
	}

	// разворачиваем — запрос DESC, клиент хочет ASC (старые сверху)
	result := make([]MessageWithUser, len(rows))
	for i, r := range rows {
		result[len(rows)-1-i] = MessageWithUser{
			ID:         r.ID,
			EventID:    r.EventID,
			UserID:     r.UserID,
			Text:       r.Text,
			ReplyToID:  r.ReplyToID, // уже *uuid.UUID
			CreatedAt:  r.CreatedAt, // time.Time
			UserName:   r.UserName,
			UserAvatar: r.UserAvatar,
		}
	}
	return result, nil
}

func (s *ChatService) MarkRead(ctx context.Context, eventID, userID uuid.UUID) error {
	return s.db.MarkChatRead(ctx, dbgen.MarkChatReadParams{
		UserID: userID, EventID: eventID,
	})
}

func (s *ChatService) UnreadCount(ctx context.Context, eventID, userID uuid.UUID) (int64, error) {
	return s.db.CountUnreadMessages(ctx, dbgen.CountUnreadMessagesParams{
		EventID: eventID, UserID: userID,
	})
}
