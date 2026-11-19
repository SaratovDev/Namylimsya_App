package services

import (
	"context"
	"encoding/json"
	"errors"

	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
)

var (
	ErrAdminUserNotFound       = errors.New("Пользователь не найден")
	ErrCannotBanSelf           = errors.New("Нельзя забанить самого себя")
	ErrModeratorCannotBanAdmin = errors.New("Модератор не может забанить администратора")
	ErrBanReasonRequired       = errors.New("Причина бана обязательна")
	ErrBanExpiresInPast        = errors.New("Дата окончания бана должна быть в будущем")
	ErrInvalidRole             = errors.New("Недопустимая роль")
	ErrEventAlreadyCancelled   = errors.New("Событие уже отменено")
)

type AdminService struct {
	db        *dbgen.Queries
	pool      *pgxpool.Pool
	sm        *auth.SessionManager
	notif     *NotificationService
	accessTTL time.Duration
}

func NewAdminService(db *dbgen.Queries, pool *pgxpool.Pool, sm *auth.SessionManager, notif *NotificationService, accessTTL time.Duration) *AdminService {
	return &AdminService{db: db, pool: pool, sm: sm, notif: notif, accessTTL: accessTTL}
}

type Dashboard struct {
	Users         dbgen.AdminDashboardUsersRow        `json:"users"`
	Events        dbgen.AdminDashboardEventsRow       `json:"events"`
	Applications  dbgen.AdminDashboardApplicationsRow `json:"applications"`
	TopCategories []dbgen.AdminTopCategoriesRow       `json:"top_categories"`
	TopOrganizers []dbgen.AdminTopOrganizersRow       `json:"top_organizers"`
	Activity      []dbgen.AdminActivityChartRow       `json:"activity"`
}

func (s *AdminService) GetDashboard(ctx context.Context) (*Dashboard, error) {
	users, err := s.db.AdminDashboardUsers(ctx)
	if err != nil {
		return nil, err
	}
	events, err := s.db.AdminDashboardEvents(ctx)
	if err != nil {
		return nil, err
	}
	apps, err := s.db.AdminDashboardApplications(ctx)
	if err != nil {
		return nil, err
	}
	cats, err := s.db.AdminTopCategories(ctx)
	if err != nil {
		return nil, err
	}
	orgs, err := s.db.AdminTopOrganizers(ctx)
	if err != nil {
		return nil, err
	}
	act, err := s.db.AdminActivityChart(ctx)
	if err != nil {
		return nil, err
	}

	return &Dashboard{
		Users: users, Events: events, Applications: apps,
		TopCategories: cats, TopOrganizers: orgs, Activity: act,
	}, nil
}

func (s *AdminService) ListUsers(ctx context.Context, search *string, role *string, limit, offset int32) ([]dbgen.AdminListUsersRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.db.AdminListUsers(ctx, dbgen.AdminListUsersParams{
		Search: search, Role: role, Limit: limit, Offset: offset,
	})
}

// ─── User details ─────────────────────────────────────────────────────

type UserDetailsResponse struct {
	User  UserPublicView `json:"user"`
	Stats UserStats      `json:"stats"`
}

type UserPublicView struct {
	ID             uuid.UUID   `json:"id"`
	Email          string      `json:"email"`
	Username       string      `json:"username"`
	Gender         *string     `json:"gender"`
	BirthDate      pgtype.Date `json:"birth_date"`
	ShowAge        bool        `json:"show_age"`
	City           *string     `json:"city"`
	Bio            *string     `json:"bio"`
	Hobbies        []string    `json:"hobbies"`
	Telegram       *string     `json:"telegram"`
	VK             *string     `json:"vk"`
	AvatarURL      *string     `json:"avatar_url"`
	Role           string      `json:"role"`
	IsBanned       bool        `json:"is_banned"`
	BanReason      *string     `json:"ban_reason"`
	BanExpiresAt   *time.Time  `json:"ban_expires_at"`
	EmailVerified  bool        `json:"email_verified"`
	ConsentAt      *time.Time  `json:"consent_at"`
	ConsentVersion *string     `json:"consent_version"`
	AgeConfirmedAt *time.Time  `json:"age_confirmed_at"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type UserStats struct {
	OrganizedEvents       int64 `json:"organized_events"`
	ApplicationsTotal     int64 `json:"applications_total"`
	ApplicationsApproved  int64 `json:"applications_approved"`
	ApplicationsPending   int64 `json:"applications_pending"`
	ApplicationsRejected  int64 `json:"applications_rejected"`
	ApplicationsCancelled int64 `json:"applications_cancelled"`
}

func (s *AdminService) GetUserDetails(ctx context.Context, userID uuid.UUID) (*UserDetailsResponse, error) {
	row, err := s.db.AdminGetUserDetails(ctx, userID)
	if err != nil {
		return nil, ErrAdminUserNotFound
	}
	return &UserDetailsResponse{
		User: UserPublicView{
			ID: row.ID, Email: row.Email, Username: row.Username,
			Gender: row.Gender, BirthDate: row.BirthDate, ShowAge: row.ShowAge,
			City: row.City, Bio: row.Bio, Hobbies: row.Hobbies,
			Telegram: row.Telegram, VK: row.Vk, AvatarURL: row.AvatarUrl,
			Role: row.Role, IsBanned: row.IsBanned,
			BanReason: row.BanReason, BanExpiresAt: row.BanExpiresAt,
			EmailVerified: row.EmailVerified,
			ConsentAt:     row.ConsentAt, ConsentVersion: row.ConsentVersion,
			AgeConfirmedAt: row.AgeConfirmedAt,
			CreatedAt:      row.CreatedAt, UpdatedAt: row.UpdatedAt,
		},
		Stats: UserStats{
			OrganizedEvents:       row.OrganizedEventsCount,
			ApplicationsTotal:     row.ApplicationsTotal,
			ApplicationsApproved:  row.ApplicationsApproved,
			ApplicationsPending:   row.ApplicationsPending,
			ApplicationsRejected:  row.ApplicationsRejected,
			ApplicationsCancelled: row.ApplicationsCancelled,
		},
	}, nil
}

// ─── Role change ──────────────────────────────────────────────────────

func (s *AdminService) UpdateUserRole(ctx context.Context, adminID, userID uuid.UUID, role string) (*dbgen.AdminUpdateUserRoleRow, error) {
	valid := map[string]bool{"user": true, "moderator": true, "admin": true}
	if !valid[role] {
		return nil, ErrInvalidRole
	}

	old, err := s.db.AdminGetUserRole(ctx, userID)
	if err != nil {
		return nil, ErrAdminUserNotFound
	}

	u, err := s.db.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: userID, Role: role,
	})
	if err != nil {
		return nil, err
	}

	// Смена роли не инвалидирует access-токен (stale role — известное ограничение,
	// фикс в 2.x). Но audit log пишем.
	s.writeAudit(ctx, adminID, "change_role", "user", userID, map[string]any{
		"old_role": old.Role,
		"new_role": role,
	})

	return &u, nil
}

// ─── Ban ──────────────────────────────────────────────────────────────

func (s *AdminService) BanUser(
	ctx context.Context,
	adminID uuid.UUID,
	userID uuid.UUID,
	banned bool,
	reason string,
	expiresAt *time.Time,
) (*dbgen.AdminBanUserRow, error) {
	if adminID == userID {
		return nil, ErrCannotBanSelf
	}

	target, err := s.db.AdminGetUserRole(ctx, userID)
	if err != nil {
		return nil, ErrAdminUserNotFound
	}

	// Модератор не может банить админа.
	if banned {
		admin, err := s.db.AdminGetUserRole(ctx, adminID)
		if err != nil {
			return nil, err
		}
		if admin.Role == "moderator" && target.Role == "admin" {
			return nil, ErrModeratorCannotBanAdmin
		}
	}

	// Валидация при бане.
	var reasonPtr *string
	var expiresPtr *time.Time
	if banned {
		reason = trimSpace(reason)
		if reason == "" {
			return nil, ErrBanReasonRequired
		}
		reasonPtr = &reason
		if expiresAt != nil {
			if expiresAt.Before(time.Now()) {
				return nil, ErrBanExpiresInPast
			}
			expiresPtr = expiresAt
		}
	}

	u, err := s.db.AdminBanUser(ctx, dbgen.AdminBanUserParams{
		ID:           userID,
		IsBanned:     banned,
		BanReason:    reasonPtr,
		BanExpiresAt: expiresPtr,
	})
	if err != nil {
		return nil, err
	}

	// Refresh + Redis-флаг.
	if s.sm != nil {
		if banned {
			if err := s.sm.RevokeAllForUser(ctx, userID); err != nil {
				log.Warn().Err(err).Str("user_id", userID.String()).
					Msg("failed to revoke refresh tokens on ban")
			}

			// TTL флага в Redis. Логика:
			//   - бессрочный бан: ~100 лет — флаг снимется только вручную (unban);
			//   - временный бан: остаток срока + 5 минут запаса, но не меньше
			//     access TTL, иначе юзер с ещё валидным access-токеном пройдёт
			//     middleware в окне между истечением флага и фоновым разбаном.
			ttl := 100 * 365 * 24 * time.Hour
			if expiresPtr != nil {
				remaining := time.Until(*expiresPtr)
				if remaining < 0 {
					remaining = 0
				}
				ttl = remaining + 5*time.Minute
				if ttl < s.accessTTL {
					ttl = s.accessTTL
				}
			}
			_ = s.sm.MarkBanned(ctx, userID, ttl)
		} else {
			_ = s.sm.ClearBanned(ctx, userID)
		}
	}

	action := "unban_user"
	if banned {
		action = "ban_user"
	}
	details := map[string]any{"banned": banned}
	if reasonPtr != nil {
		details["reason"] = *reasonPtr
	}
	if expiresPtr != nil {
		details["expires_at"] = expiresPtr.Format(time.RFC3339)
	}
	s.writeAudit(ctx, adminID, action, "user", userID, details)

	return &u, nil
}

// ─── Audit log ────────────────────────────────────────────────────────

// writeAudit пишет в audit log. Не возвращает ошибку — лог не должен
// ломать основную операцию.
func (s *AdminService) writeAudit(
	ctx context.Context,
	adminID uuid.UUID,
	action, targetType string,
	targetID uuid.UUID,
	details map[string]any,
) {
	admin, err := s.db.AdminGetUserRole(ctx, adminID)
	if err != nil {
		// всё равно пишем — с пустым email (не должно случаться)
		admin = dbgen.AdminGetUserRoleRow{ID: adminID}
	}
	email, _ := s.getUserEmail(ctx, adminID)

	raw, _ := json.Marshal(details)
	rawMsg := json.RawMessage(raw)

	adminIDCopy := admin.ID
	_, err = s.db.AdminInsertAuditLog(ctx, dbgen.AdminInsertAuditLogParams{
		AdminID:    &adminIDCopy,
		AdminEmail: email,
		Action:     action,
		TargetType: targetType,
		TargetID:   &targetID,
		Details:    &rawMsg,
	})

	if err != nil {
		log.Error().Err(err).
			Str("action", action).
			Str("target_type", targetType).
			Str("target_id", targetID.String()).
			Str("admin_id", adminIDCopy.String()).
			Msg("AUDIT INSERT FAILED") // ← изменил Warn на Error + подробнее
	}
}

func (s *AdminService) getUserEmail(ctx context.Context, userID uuid.UUID) (string, error) {
	row, err := s.db.AdminGetUserDetails(ctx, userID)
	if err != nil {
		return "", err
	}
	return row.Email, nil
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// ─── Existing ─────────────────────────────────────────────────────────
// ─── Event details / cancel / hide ────────────────────────────────────

type AdminEventDetails struct {
	Event        dbgen.AdminGetEventDetailsRow     `json:"event"`
	Applications []dbgen.ListEventApplicationsRow  `json:"applications"`
	Messages     []dbgen.AdminGetRecentMessagesRow `json:"messages"`
}

func (s *AdminService) GetEventDetails(ctx context.Context, eventID uuid.UUID) (*AdminEventDetails, error) {
	row, err := s.db.AdminGetEventDetails(ctx, eventID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEventNotFound
		}
		return nil, err
	}

	apps, err := s.db.ListEventApplications(ctx, eventID)
	if err != nil {
		apps = nil
	}

	msgs, err := s.db.AdminGetRecentMessages(ctx, dbgen.AdminGetRecentMessagesParams{
		EventID: eventID, Limit: 50,
	})
	if err != nil {
		msgs = nil
	}

	// Сообщения пришли DESC — развернём в хронологический порядок.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return &AdminEventDetails{
		Event: row, Applications: apps, Messages: msgs,
	}, nil
}

func (s *AdminService) CancelEvent(ctx context.Context, adminID, eventID uuid.UUID, reason string) (*dbgen.Event, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	qtx := dbgen.New(tx)

	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	event, err := qtx.AdminCancelEvent(ctx, dbgen.AdminCancelEventParams{
		ID: eventID, CancelReason: reasonPtr,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Различаем «не найдено» / «уже неактивно».
			if _, getErr := s.db.GetEventByID(ctx, dbgen.GetEventByIDParams{ID: eventID}); getErr != nil {
				return nil, ErrEventNotFound
			}
			return nil, ErrEventNotActive
		}
		return nil, err
	}

	// Собираем активные заявки до отмены — внутри той же транзакции.
	activeApps, err := qtx.ListActiveApplicationsForEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}

	// Каскадная отмена.
	if err := qtx.CancelAllActiveApplicationsForEvent(ctx, dbgen.CancelAllActiveApplicationsForEventParams{
		EventID:      eventID,
		CancelReason: reasonPtr,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Уведомления — после коммита.
	if s.notif != nil {
		for _, app := range activeApps {
			payload := map[string]any{
				"event_id":       eventID,
				"event_title":    event.Title,
				"application_id": app.ID,
				"admin_action":   true,
			}
			if reason != "" {
				payload["reason"] = reason
			}
			if _, err := s.notif.Create(ctx, CreateInput{
				UserID:    app.UserID,
				Type:      "event_cancelled",
				Payload:   payload,
				SendEmail: true,
			}); err != nil {
				log.Warn().Err(err).
					Str("event_id", eventID.String()).
					Str("application_id", app.ID.String()).
					Msg("admin cancel: failed to notify")
			}
		}
	}

	details := map[string]any{}
	if reason != "" {
		details["reason"] = reason
	}
	s.writeAudit(ctx, adminID, "cancel_event", "event", eventID, details)

	// sqlc сгенерировал AdminCancelEventRow вместо Event — конвертируем.
	ev := dbgen.Event(event)
	return &ev, nil
}

func (s *AdminService) HideEvent(ctx context.Context, adminID, eventID uuid.UUID) error {
	if _, err := s.db.AdminHideEvent(ctx, eventID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEventNotFound
		}
		return err
	}
	s.writeAudit(ctx, adminID, "hide_event", "event", eventID, nil)
	return nil
}

func (s *AdminService) UnhideEvent(ctx context.Context, adminID, eventID uuid.UUID) error {
	if _, err := s.db.AdminUnhideEvent(ctx, eventID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEventNotFound
		}
		return err
	}
	s.writeAudit(ctx, adminID, "unhide_event", "event", eventID, nil)
	return nil
}

func (s *AdminService) ListEvents(
	ctx context.Context,
	status, search, category, city *string,
	limit, offset int32,
) ([]dbgen.AdminListEventsRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.db.AdminListEvents(ctx, dbgen.AdminListEventsParams{
		Status:   status,
		Search:   search,
		Category: category,
		City:     city,
		Limit:    limit,
		Offset:   offset,
	})
}

func (s *AdminService) DeleteEvent(ctx context.Context, adminID, id uuid.UUID) error {
	if err := s.db.AdminDeleteEvent(ctx, id); err != nil {
		return err
	}
	s.writeAudit(ctx, adminID, "delete_event", "event", id, nil)
	return nil
}

// ─── Audit log list (для A.6, роут пока не подключён) ────────────────

func (s *AdminService) ListAuditLog(
	ctx context.Context,
	adminID *uuid.UUID,
	action *string,
	targetType *string,
	limit, offset int32,
) ([]dbgen.AdminAuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.db.AdminListAuditLog(ctx, dbgen.AdminListAuditLogParams{
		AdminID: adminID, Action: action, TargetType: targetType,
		Limit: limit, Offset: offset,
	})
}

// Audit — публичная обёртка над writeAudit для других сервисов
// (например, SupportService для логов resolve/reject).
func (s *AdminService) Audit(
	ctx context.Context,
	adminID uuid.UUID,
	action, targetType string,
	targetID uuid.UUID,
	details map[string]any,
) {
	s.writeAudit(ctx, adminID, action, targetType, targetID, details)
}
