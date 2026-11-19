package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/email"
	"github.com/SaratovDev/Namylimsya_App/api/internal/legal"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
)

var (
	ErrUserExists        = errors.New("Такой пользователь уже существует")
	ErrInvalidCreds      = errors.New("Данные введены неверно")
	ErrUserBanned        = errors.New("Ну и ну! Это же бан!")
	ErrConsentRequired   = errors.New("Требуется согласие на обработку персональных данных")
	ErrAgeRequired       = errors.New("Малышам нельзя, у нас тут 18+")
	ErrConsentVersionOld = errors.New("Версия документов устарела. Обновите страницу и попробуйте снова")

	// Password reset:
	ErrInvalidResetCode = errors.New("Неверный или истёкший код")
	ErrTooManyAttempts  = errors.New("Слишком много попыток. Подождите 15 минут.")

	// Email verification:
	ErrInvalidVerificationToken = errors.New("Неверная или истёкшая ссылка подтверждения")
)

type AuthService struct {
	db  *dbgen.Queries
	tm  *auth.TokenManager
	sm  *auth.SessionManager
	rdb *redis.Client
}

// RequestPasswordReset — генерирует 6-значный код, сохраняет в Redis (TTL 15 мин),
// отправляет email. Всегда возвращает nil (даже если email не найден) — чтобы
// не палить, зарегистрирован ли email (enumeration protection).
func (s *AuthService) RequestPasswordReset(ctx context.Context, emailAddr string, mailer *email.Sender) error {
	emailAddr = strings.ToLower(strings.TrimSpace(emailAddr))

	// Anti-abuse: не чаще 1 раза в 60 сек на email.
	throttleKey := fmt.Sprintf("pwreset:throttle:%s", emailAddr)
	ok, err := s.rdbSetNX(ctx, throttleKey, "1", 60*time.Second)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	user, err := s.db.GetUserByEmail(ctx, emailAddr)
	if err != nil {
		return nil
	}

	code, err := randomSixDigit()
	if err != nil {
		return err
	}

	codeKey := fmt.Sprintf("pwreset:code:%s", emailAddr)
	if err := s.rdbSet(ctx, codeKey, code, 15*time.Minute); err != nil {
		return err
	}

	if mailer != nil {
		body := fmt.Sprintf(`
<p>Здравствуйте, %s!</p>
<p>Вы запросили восстановление пароля на «Намылимся».</p>
<p><b>Код: %s</b></p>
<p>Введите его на странице восстановления. Код действует 15 минут.</p>
<p>Если вы не запрашивали — просто проигнорируйте это письмо.</p>
`, user.Username, code)

		_ = mailer.Send(ctx, email.Message{
			To:      emailAddr,
			Subject: "Восстановление пароля — Намылимся",
			Body:    body,
		})
	}

	return nil
}

// ResetPassword — проверяет код, меняет пароль, отзывает все сессии.
func (s *AuthService) ResetPassword(ctx context.Context, emailAddr, code, newPassword string) error {
	emailAddr = strings.ToLower(strings.TrimSpace(emailAddr))

	if len(newPassword) < 8 {
		return errors.New("Пароль должен быть не короче 8 символов")
	}
	if len(newPassword) > 72 {
		return errors.New("Пароль слишком длинный (макс. 72 символа)")
	}

	attemptsKey := fmt.Sprintf("pwreset:attempts:%s", emailAddr)
	attempts, err := s.rdbIncr(ctx, attemptsKey, 15*time.Minute)
	if err != nil {
		return err
	}
	if attempts > 5 {
		return ErrTooManyAttempts
	}

	codeKey := fmt.Sprintf("pwreset:code:%s", emailAddr)
	storedCode, err := s.rdbGet(ctx, codeKey)
	if err != nil || storedCode == "" {
		return ErrInvalidResetCode
	}
	if storedCode != code {
		return ErrInvalidResetCode
	}

	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}

	user, err := s.db.GetUserByEmail(ctx, emailAddr)
	if err != nil {
		return ErrInvalidResetCode
	}

	if err := s.db.UpdateUserPassword(ctx, dbgen.UpdateUserPasswordParams{
		ID:           user.ID,
		PasswordHash: hash,
	}); err != nil {
		return err
	}

	if s.sm != nil {
		_ = s.sm.RevokeAllForUser(ctx, user.ID)
	}

	_ = s.rdbDel(ctx, codeKey)
	_ = s.rdbDel(ctx, attemptsKey)

	return nil
}

// ─── Redis helpers (чтобы не тащить redis.Client в auth.go) ─────
// Эти методы должны использовать *redis.Client из SessionManager.
// Если в SessionManager нет доступа к rdb — добавь геттер или используй s.sm.

func NewAuthService(db *dbgen.Queries, tm *auth.TokenManager, sm *auth.SessionManager, rdb *redis.Client) *AuthService {
	return &AuthService{db: db, tm: tm, sm: sm, rdb: rdb}
}

type RegisterInput struct {
	Email           string
	Password        string
	Username        string
	Gender          string
	BirthDate       string
	City            string
	ConsentAccepted bool
	AgeConfirmed    bool
	ConsentVersion  string
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*dbgen.User, error) {
	// Нормализация email: иначе "Foo@Bar.com" и "foo@bar.com" — разные аккаунты,
	// а reset (который нормализует) не найдёт пользователя → перманентный локаут.
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	if !input.ConsentAccepted {
		return nil, ErrConsentRequired
	}
	if !input.AgeConfirmed {
		return nil, ErrAgeRequired
	}
	if input.ConsentVersion != legal.ConsentVersion {
		return nil, ErrConsentVersionOld
	}
	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	var birthDate pgtype.Date
	if input.BirthDate != "" {
		t, err := time.Parse("2006-01-02", input.BirthDate)
		if err != nil {
			return nil, fmt.Errorf("invalid birth_date, expected YYYY-MM-DD: %w", err)
		}
		birthDate = pgtype.Date{Time: t, Valid: true}
	}

	var gender *string
	if input.Gender != "" {
		gender = &input.Gender
	}

	var city *string
	if input.City != "" {
		city = &input.City
	}

	now := time.Now()
	user, err := s.db.CreateUser(ctx, dbgen.CreateUserParams{
		Email:          input.Email,
		PasswordHash:   hash,
		Username:       input.Username,
		Gender:         gender,
		BirthDate:      birthDate,
		City:           city,
		ConsentAt:      &now,
		ConsentVersion: &input.ConsentVersion,
		AgeConfirmedAt: &now,
	})

	if err != nil {
		// Ловим ошибку уникальности (postgres code 23505)
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return &user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, string, *dbgen.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.db.GetUserByEmail(ctx, email)
	if err != nil {
		return "", "", nil, ErrInvalidCreds
	}

	if !auth.CheckPasswordHash(password, user.PasswordHash) {
		return "", "", nil, ErrInvalidCreds
	}

	// Проверяем бан ДО выдачи токенов. Бан считается активным, только если
	// ban_expires_at пуст (бессрочный) или ещё не наступил. Истёкший бан
	// не блокирует логин — фоновый воркер снимет is_banned в ближайшие 5 минут,
	// но ждать его незачем.
	if user.IsBanned && (user.BanExpiresAt == nil || user.BanExpiresAt.After(time.Now())) {
		return "", "", nil, ErrUserBanned
	}

	accessToken, err := s.tm.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		return "", "", nil, err
	}

	refreshToken, err := s.tm.GenerateRefreshToken(user.ID)
	if err != nil {
		return "", "", nil, err
	}

	// Извлекаем JTI из refresh токена, чтобы сохранить его в Redis
	claims, _ := s.tm.ParseRefreshToken(refreshToken)
	ttl := s.tm.GetRefreshTTL() // Нам понадобится геттер в TokenManager

	if err := s.sm.SaveRefreshToken(ctx, user.ID, claims.ID, ttl); err != nil {
		return "", "", nil, err
	}

	return accessToken, refreshToken, &user, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, id uuid.UUID) (*dbgen.User, error) {
	user, err := s.db.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	claims, err := s.tm.ParseRefreshToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	// Проверяем в Redis
	valid, err := s.sm.IsRefreshTokenValid(ctx, claims.UserID, claims.ID)
	if err != nil || !valid {
		return "", "", errors.New("refresh token revoked")
	}

	// Отзываем старый (ротация)
	_ = s.sm.RevokeRefreshToken(ctx, claims.UserID, claims.ID)

	// Получаем юзера для роли
	// Получаем юзера для роли и проверки бана
	user, err := s.db.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return "", "", err
	}

	// Забаненный пользователь не может обновить access-токен.
	// Отзываем его текущий refresh, чтобы не висел в Redis.
	// Истёкший временный бан не считается активным.
	if user.IsBanned && (user.BanExpiresAt == nil || user.BanExpiresAt.After(time.Now())) {
		_ = s.sm.RevokeRefreshToken(ctx, claims.UserID, claims.ID)
		return "", "", ErrUserBanned
	}

	newAccess, err := s.tm.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		return "", "", err
	}
	newRefresh, err := s.tm.GenerateRefreshToken(user.ID)
	if err != nil {
		return "", "", err
	}

	// Сохраняем новый refresh в Redis
	newClaims, _ := s.tm.ParseRefreshToken(newRefresh)
	ttl := s.tm.GetRefreshTTL()
	if err := s.sm.SaveRefreshToken(ctx, user.ID, newClaims.ID, ttl); err != nil {
		return "", "", err
	}

	return newAccess, newRefresh, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	claims, err := s.tm.ParseRefreshToken(refreshToken)
	if err != nil {
		return err
	}
	return s.sm.RevokeRefreshToken(ctx, claims.UserID, claims.ID)
}

// CreateVerificationToken — генерирует криптостойкий токен, кладёт в Redis
// (TTL 24 часа) с привязкой к userID, возвращает токен для ссылки в письме.
func (s *AuthService) CreateVerificationToken(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := randomHex(16)
	if err != nil {
		return "", err
	}
	if err := s.rdbSet(ctx, "email_verify:"+token, userID.String(), 24*time.Hour); err != nil {
		return "", err
	}
	return token, nil
}

// VerifyEmail — по токену из письма подтверждает email и удаляет токен.
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return ErrInvalidVerificationToken
	}
	key := "email_verify:" + token
	userIDStr, err := s.rdbGet(ctx, key)
	if err != nil || userIDStr == "" {
		return ErrInvalidVerificationToken
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return ErrInvalidVerificationToken
	}
	if err := s.db.MarkEmailVerified(ctx, userID); err != nil {
		return err
	}
	_ = s.rdbDel(ctx, key)
	return nil
}

func (s *AuthService) rdbSet(ctx context.Context, key, val string, ttl time.Duration) error {
	return s.rdb.Set(ctx, key, val, ttl).Err()
}

func (s *AuthService) rdbSetNX(ctx context.Context, key, val string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, key, val, ttl).Result()
}

func (s *AuthService) rdbGet(ctx context.Context, key string) (string, error) {
	val, err := s.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (s *AuthService) rdbDel(ctx context.Context, key string) error {
	return s.rdb.Del(ctx, key).Err()
}

func (s *AuthService) rdbIncr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	val, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if val == 1 {
		_ = s.rdb.Expire(ctx, key, ttl).Err()
	}
	return val, nil
}

// randomHex — n случайных байт в hex (криптостойкий, для токенов).
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// randomSixDigit — криптостойкий 6-значный код для сброса пароля.
func randomSixDigit() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	n := int(b[0])<<16 | int(b[1])<<8 | int(b[2]) // 0..16_777_215
	return fmt.Sprintf("%06d", n%900000+100000), nil
}
