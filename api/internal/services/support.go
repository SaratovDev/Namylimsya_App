package services

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/email"
)

var (
	ErrTicketNotFound    = errors.New("Тикет не найден")
	ErrTicketClosed      = errors.New("Тикет уже закрыт или отклонён")
	ErrInvalidTargetType = errors.New("Недопустимый тип цели жалобы")
	ErrInvalidTicketType = errors.New("Недопустимый тип тикета")
	ErrComplaintNotFound = errors.New("Жалоба не найдена")
	ErrComplaintClosed   = errors.New("Жалоба уже разрешена или отклонена")
	ErrMessageNotFound   = errors.New("Сообщение не найдено")

	// Валидация пользовательского ввода.
	ErrMessageEmpty     = errors.New("Сообщение не может быть пустым")
	ErrMessageTooLong   = fmt.Errorf("Сообщение слишком длинное (максимум %d символов)", maxMessageLen)
	ErrSubjectTooLong   = fmt.Errorf("Тема слишком длинная (максимум %d символов)", maxSubjectLen)
	ErrReasonEmpty      = errors.New("Причина не может быть пустой")
	ErrReasonTooLong    = fmt.Errorf("Причина слишком длинная (максимум %d символов)", maxReasonLen)
	ErrReplyBodyEmpty   = errors.New("Текст ответа не может быть пустым")
	ErrReplyBodyTooLong = fmt.Errorf("Текст ответа слишком длинный (максимум %d символов)", maxReplyBodyLen)
)

// Ограничения на пользовательский ввод. Символы (rune), не байты —
// чтобы кириллица не урезалась вдвое.
const (
	maxSubjectLen   = 300
	maxMessageLen   = 5000
	maxReasonLen    = 1000
	maxReplyBodyLen = 5000
)

type SupportService struct {
	db     *dbgen.Queries
	audit  *AdminService
	mailer *email.Sender
}

func NewSupportService(db *dbgen.Queries, audit *AdminService, mailer *email.Sender) *SupportService {
	return &SupportService{db: db, audit: audit, mailer: mailer}
}

// ─── Tickets ──────────────────────────────────────────────────────────

func (s *SupportService) CreateTicket(ctx context.Context, userID *uuid.UUID, ticketType, subject, message string) (*dbgen.SupportTicket, error) {
	validTypes := map[string]bool{"complaint": true, "suggestion": true, "appeal": true}
	if !validTypes[ticketType] {
		return nil, ErrInvalidTicketType
	}
	if len([]rune(subject)) > maxSubjectLen {
		return nil, ErrSubjectTooLong
	}
	if strings.TrimSpace(message) == "" {
		return nil, ErrMessageEmpty
	}
	if len([]rune(message)) > maxMessageLen {
		return nil, ErrMessageTooLong
	}

	t, err := s.db.CreateSupportTicket(ctx, dbgen.CreateSupportTicketParams{
		UserID:  userID,
		Type:    ticketType,
		Subject: subject,
		Message: message,
	})
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *SupportService) ListTickets(ctx context.Context, status *string, limit, offset int32) ([]dbgen.ListSupportTicketsRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.db.ListSupportTickets(ctx, dbgen.ListSupportTicketsParams{
		Status: status, Limit: limit, Offset: offset,
	})
}

// ─── Ticket details ──────────────────────────────────────────────────

type TicketDetails struct {
	Ticket  dbgen.GetSupportTicketRow `json:"ticket"`
	Replies []dbgen.TicketReply       `json:"replies"`
}

func (s *SupportService) GetTicket(ctx context.Context, id uuid.UUID) (*TicketDetails, error) {
	t, err := s.db.GetSupportTicket(ctx, id)
	if err != nil {
		return nil, ErrTicketNotFound
	}
	replies, _ := s.db.ListTicketReplies(ctx, id)
	if replies == nil {
		replies = []dbgen.TicketReply{}
	}
	return &TicketDetails{Ticket: t, Replies: replies}, nil
}

// ─── Ticket actions ──────────────────────────────────────────────────

func (s *SupportService) ResolveTicket(ctx context.Context, adminID, id uuid.UUID, comment string) (*dbgen.SupportTicket, error) {
	var commentPtr *string
	if strings.TrimSpace(comment) != "" {
		commentPtr = &comment
	}

	t, err := s.db.ResolveSupportTicket(ctx, dbgen.ResolveSupportTicketParams{
		ID:                id,
		ResolutionComment: commentPtr,
		ResolvedBy:        &adminID,
	})
	if err != nil {
		return nil, s.ticketUpdateError(ctx, id)
	}

	if s.audit != nil {
		s.audit.Audit(ctx, adminID, "resolve_ticket", "ticket", id, map[string]any{
			"comment": comment,
		})
	}
	return &t, nil
}

func (s *SupportService) RejectTicket(ctx context.Context, adminID, id uuid.UUID, comment string) (*dbgen.SupportTicket, error) {
	var commentPtr *string
	if strings.TrimSpace(comment) != "" {
		commentPtr = &comment
	}

	t, err := s.db.RejectSupportTicket(ctx, dbgen.RejectSupportTicketParams{
		ID:                id,
		ResolutionComment: commentPtr,
		ResolvedBy:        &adminID,
	})
	if err != nil {
		return nil, s.ticketUpdateError(ctx, id)
	}

	if s.audit != nil {
		s.audit.Audit(ctx, adminID, "reject_ticket", "ticket", id, map[string]any{
			"comment": comment,
		})
	}
	return &t, nil
}

// ticketUpdateError различает «не найдено» и «уже закрыто».
func (s *SupportService) ticketUpdateError(ctx context.Context, id uuid.UUID) error {
	if _, getErr := s.db.GetSupportTicket(ctx, id); getErr != nil {
		return ErrTicketNotFound
	}
	return ErrTicketClosed
}

// ReplyToTicket пишет ответ в ticket_replies и (если известен email) отправляет письмо.
func (s *SupportService) ReplyToTicket(
	ctx context.Context,
	adminID uuid.UUID,
	ticketID uuid.UUID,
	body string,
) (*dbgen.TicketReply, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, ErrReplyBodyEmpty
	}
	if len([]rune(body)) > maxReplyBodyLen {
		return nil, ErrReplyBodyTooLong
	}

	t, err := s.db.GetSupportTicket(ctx, ticketID)
	if err != nil {
		return nil, ErrTicketNotFound
	}

	// Email админа — снимок на момент ответа.
	adminEmail := ""
	if adminDetails, err := s.auditGetUserEmail(ctx, adminID); err == nil {
		adminEmail = adminDetails
	}

	reply, err := s.db.CreateTicketReply(ctx, dbgen.CreateTicketReplyParams{
		TicketID:   ticketID,
		AdminID:    &adminID,
		AdminEmail: adminEmail,
		Body:       body,
	})
	if err != nil {
		return nil, err
	}

	// Отправка email пользователю (если известен и mailer доступен).
	if s.mailer != nil && t.UserEmail != nil && *t.UserEmail != "" {
		// Subject содержит t.Subject — пользовательский ввод из тикета.
		// email.Send делает SanitizeHeader, но продублируем здесь, чтобы
		// "Ответ: " склеился уже с чистой строкой.
		subject := "Ответ на обращение в поддержку"
		if t.Subject != "" {
			subject = "Ответ: " + email.SanitizeHeader(t.Subject)
		}
		// body — текст админа. Не доверяем HTML-контексту: экранируем,
		// затем \n → <br>, чтобы переносы сохранились.
		escapedBody := html.EscapeString(body)
		escapedBody = strings.ReplaceAll(escapedBody, "\n", "<br>")
		if err := s.mailer.Send(ctx, email.Message{
			To:      *t.UserEmail,
			Subject: subject,
			Body:    "<p>" + escapedBody + "</p>",
		}); err != nil {
			log.Warn().Err(err).
				Str("ticket_id", ticketID.String()).
				Str("to", *t.UserEmail).
				Msg("failed to send ticket reply email")
		}
	}

	if s.audit != nil {
		s.audit.Audit(ctx, adminID, "reply_ticket", "ticket", ticketID, map[string]any{
			"reply_id": reply.ID.String(),
		})
	}
	return &reply, nil
}

// auditGetUserEmail — вытащить email админа для снимка в ticket_replies.
func (s *SupportService) auditGetUserEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	u, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return u.Email, nil
}

func (s *SupportService) CountOpenTickets(ctx context.Context) (int64, error) {
	return s.db.CountSupportTickets(ctx)
}

// ─── Complaints ───────────────────────────────────────────────────────

func (s *SupportService) CreateComplaint(ctx context.Context, reporterID uuid.UUID, targetType string, targetID uuid.UUID, reason string) (*dbgen.Complaint, error) {
	valid := map[string]bool{"event": true, "user": true, "message": true}
	if !valid[targetType] {
		return nil, ErrInvalidTargetType
	}
	if strings.TrimSpace(reason) == "" {
		return nil, ErrReasonEmpty
	}
	if len([]rune(reason)) > maxReasonLen {
		return nil, ErrReasonTooLong
	}

	c, err := s.db.CreateComplaint(ctx, dbgen.CreateComplaintParams{
		ReporterID: reporterID,
		TargetType: targetType,
		TargetID:   targetID,
		Reason:     reason,
	})
	if err != nil {
		return nil, err
	}

	// Автоматически создаём тикет для модерации.
	subject := "Жалоба на " + complaintTargetLabel(targetType)
	if _, err := s.db.CreateComplaintSupportTicket(ctx, dbgen.CreateComplaintSupportTicketParams{
		UserID:      &reporterID,
		Subject:     subject,
		Message:     reason,
		ComplaintID: &c.ID,
	}); err != nil {
		log.Warn().Err(err).
			Str("complaint_id", c.ID.String()).
			Msg("failed to create ticket for complaint")
		// не возвращаем ошибку — жалоба создана, тикет не критичен
	}

	return &c, nil
}

func complaintTargetLabel(targetType string) string {
	switch targetType {
	case "event":
		return "событие"
	case "user":
		return "пользователя"
	case "message":
		return "сообщение"
	}
	return targetType
}

func (s *SupportService) ListComplaints(ctx context.Context, status *string, limit, offset int32) ([]dbgen.ListComplaintsRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.db.ListComplaints(ctx, dbgen.ListComplaintsParams{
		Status: status, Limit: limit, Offset: offset,
	})
}

// ─── Complaint details ────────────────────────────────────────────────

type ComplaintDetails struct {
	Complaint *dbgen.GetComplaintRow                   `json:"complaint"`
	Event     *dbgen.AdminGetComplaintTargetEventRow   `json:"event,omitempty"`
	User      *dbgen.AdminGetComplaintTargetUserRow    `json:"user,omitempty"`
	Message   *dbgen.AdminGetComplaintTargetMessageRow `json:"message,omitempty"`
}

func (s *SupportService) GetComplaint(ctx context.Context, id uuid.UUID) (*ComplaintDetails, error) {
	c, err := s.db.GetComplaint(ctx, id)
	if err != nil {
		return nil, ErrComplaintNotFound
	}

	d := &ComplaintDetails{Complaint: &c}

	switch c.TargetType {
	case "event":
		e, err := s.db.AdminGetComplaintTargetEvent(ctx, c.TargetID)
		if err == nil {
			d.Event = &e
		}
	case "user":
		u, err := s.db.AdminGetComplaintTargetUser(ctx, c.TargetID)
		if err == nil {
			d.User = &u
		}
	case "message":
		m, err := s.db.AdminGetComplaintTargetMessage(ctx, c.TargetID)
		if err == nil {
			d.Message = &m
		}
	}

	return d, nil
}

// ─── Resolve / Reject complaint ──────────────────────────────────────

func (s *SupportService) ResolveComplaint(ctx context.Context, adminID, id uuid.UUID, comment string) (*dbgen.Complaint, error) {
	var commentPtr *string
	if strings.TrimSpace(comment) != "" {
		commentPtr = &comment
	}

	c, err := s.db.ResolveComplaint(ctx, dbgen.ResolveComplaintParams{
		ID:                id,
		ResolutionComment: commentPtr,
		ResolvedBy:        &adminID,
	})
	if err != nil {
		return nil, s.complaintUpdateError(ctx, id)
	}

	if s.audit != nil {
		s.audit.Audit(ctx, adminID, "resolve_complaint", "complaint", id, map[string]any{
			"comment": comment,
		})
	}
	return &c, nil
}

func (s *SupportService) RejectComplaint(ctx context.Context, adminID, id uuid.UUID, comment string) (*dbgen.Complaint, error) {
	var commentPtr *string
	if strings.TrimSpace(comment) != "" {
		commentPtr = &comment
	}

	c, err := s.db.RejectComplaint(ctx, dbgen.RejectComplaintParams{
		ID:                id,
		ResolutionComment: commentPtr,
		ResolvedBy:        &adminID,
	})
	if err != nil {
		return nil, s.complaintUpdateError(ctx, id)
	}

	if s.audit != nil {
		s.audit.Audit(ctx, adminID, "reject_complaint", "complaint", id, map[string]any{
			"comment": comment,
		})
	}
	return &c, nil
}

func (s *SupportService) complaintUpdateError(ctx context.Context, id uuid.UUID) error {
	if _, getErr := s.db.GetComplaint(ctx, id); getErr != nil {
		return ErrComplaintNotFound
	}
	return ErrComplaintClosed
}

// ─── Messages ─────────────────────────────────────────────────────────

func (s *SupportService) DeleteMessage(ctx context.Context, adminID, messageID uuid.UUID) error {
	if _, err := s.db.AdminSoftDeleteMessage(ctx, messageID); err != nil {
		return ErrMessageNotFound
	}
	if s.audit != nil {
		s.audit.Audit(ctx, adminID, "delete_message", "message", messageID, nil)
	}
	return nil
}
