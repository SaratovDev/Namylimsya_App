package handlers

import (
	"encoding/json"
	"errors"
	"html"
	"net/http"

	"github.com/SaratovDev/Namylimsya_App/api/internal/config"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/email"
	"github.com/SaratovDev/Namylimsya_App/api/internal/httpx"
	"github.com/SaratovDev/Namylimsya_App/api/internal/middleware"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
	"github.com/rs/zerolog/log"
)

type AuthHandler struct {
	svc    *services.AuthService
	cfg    *config.Config
	mailer *email.Sender
}

func NewAuthHandler(svc *services.AuthService, cfg *config.Config, mailer *email.Sender) *AuthHandler {
	return &AuthHandler{svc: svc, cfg: cfg, mailer: mailer}
}

type registerRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	Username        string `json:"username"`
	Gender          string `json:"gender,omitempty"`
	BirthDate       string `json:"birth_date,omitempty"`
	City            string `json:"city,omitempty"`
	ConsentAccepted bool   `json:"consent_accepted"`
	AgeConfirmed    bool   `json:"age_confirmed"`
	ConsentVersion  string `json:"consent_version"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	// Базовая валидация (в будущем можно вынести в отдельный пакет)
	if req.Email == "" || req.Password == "" || req.Username == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Нужны почта, пароль, и юзернейм")
		return
	}
	if len(req.Password) < 8 {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Ну ты хотя бы 8 символов сделай, это же твоя безопасность")
		return
	}

	user, err := h.svc.Register(r.Context(), services.RegisterInput{
		Email:           req.Email,
		Password:        req.Password,
		Username:        req.Username,
		Gender:          req.Gender,
		BirthDate:       req.BirthDate,
		City:            req.City,
		ConsentAccepted: req.ConsentAccepted,
		AgeConfirmed:    req.AgeConfirmed,
		ConsentVersion:  req.ConsentVersion,
	})

	if err != nil {
		if err != nil {
			if errors.Is(err, services.ErrConsentRequired) ||
				errors.Is(err, services.ErrAgeRequired) ||
				errors.Is(err, services.ErrConsentVersionOld) {
				httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
				return
			}
			if errors.Is(err, services.ErrUserExists) {
				httpx.Error(w, http.StatusConflict, "conflict", "user already exists")
				return
			}
			// ... остальное как было
		}
		if errors.Is(err, services.ErrUserExists) {
			httpx.Error(w, http.StatusConflict, "conflict", "user already exists")
			return
		}
		log.Error().Err(err).Msg("register failed")
		httpx.InternalError(w, err)
		return
	}

	// Если верификация включена — отправляем письмо со ссылкой подтверждения.
	if h.cfg.EmailVerificationEnabled {
		token, err := h.svc.CreateVerificationToken(r.Context(), user.ID)
		if err != nil {
			log.Error().Err(err).Str("user_id", user.ID.String()).Msg("create verification token")
		} else {
			link := h.cfg.PublicBaseURL + "/verify-email?token=" + token
			body := "<p>Здравствуйте, " + html.EscapeString(user.Username) + "!</p>" +
				"<p>Подтвердите email, чтобы ваш аккаунт на «Намылимся» был защищён.</p>" +
				"<p><a href=\"" + html.EscapeString(link) + "\">Подтвердить email</a></p>" +
				"<p>Ссылка действует 24 часа. Если вы не регистрировались — проигнорируйте письмо.</p>"
			_ = h.mailer.Send(r.Context(), email.Message{
				To:      user.Email,
				Subject: "Подтвердите email — Намылимся",
				Body:    body,
			})
		}
	}

	httpx.JSON(w, http.StatusCreated, map[string]any{
		"id":       user.ID,
		"email":    user.Email,
		"username": user.Username,
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	accessToken, refreshToken, _, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrUserBanned) {
			httpx.Error(w, http.StatusForbidden, "banned", "account is banned")
			return
		}
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid email or password")
		return
	}

	// В MVP refresh токен кладем в httpOnly cookie (безопаснее для веба)
	// Refresh-токен в httpOnly cookie: Path узкий, Secure в проде, MaxAge = RefreshTTL.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.cfg.IsProd(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.cfg.JWT.RefreshTTL.Seconds()),
	})

	httpx.JSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "no user in context")
		return
	}

	user, err := h.svc.GetUserByID(r.Context(), userID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}

	httpx.JSON(w, http.StatusOK, toPublicUser(user))
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "no refresh token")
		return
	}

	accessToken, newRefresh, err := h.svc.Refresh(r.Context(), cookie.Value)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid refresh token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    newRefresh,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.cfg.IsProd(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.cfg.JWT.RefreshTTL.Seconds()),
	})

	httpx.OK(w, map[string]string{"access_token": accessToken})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		_ = h.svc.Logout(r.Context(), cookie.Value)
	}
	// Удаление cookie: Name+Path+Secure+SameSite ДОЛЖНЫ совпасть с установкой,
	// иначе браузер не удалит cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		MaxAge:   -1,
		Secure:   h.cfg.IsProd(),
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func toPublicUser(u *dbgen.User) map[string]any {
	return map[string]any{
		"id":                  u.ID,
		"email":               u.Email,
		"username":            u.Username,
		"gender":              u.Gender,
		"birth_date":          u.BirthDate,
		"show_age":            u.ShowAge,
		"city":                u.City,
		"bio":                 u.Bio,
		"hobbies":             u.Hobbies,
		"telegram":            u.Telegram,
		"vk":                  u.Vk,
		"avatar_url":          u.AvatarUrl,
		"role":                u.Role,
		"email_verified":      u.EmailVerified,
		"username_changed_at": u.UsernameChangedAt,
		"created_at":          u.CreatedAt,
		"updated_at":          u.UpdatedAt,
	}
}

// ─── Email verification ────────────────────────────────────────────

// VerifyEmail — GET /auth/verify-email?token=...
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if err := h.svc.VerifyEmail(r.Context(), token); err != nil {
		if errors.Is(err, services.ErrInvalidVerificationToken) {
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, map[string]string{"message": "Email подтверждён"})
}

// ResendVerification — POST /auth/resend-verification (авторизованный).
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	user, err := h.svc.GetUserByID(r.Context(), userID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if user.EmailVerified {
		httpx.OK(w, map[string]string{"message": "Email уже подтверждён"})
		return
	}

	// Анти-спам: не чаще 1 раза в 60 сек на юзера.
	allowed, err := h.svc.ThrottleVerificationResend(r.Context(), userID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if !allowed {
		httpx.Error(w, http.StatusTooManyRequests, "rate_limit", "Слишком часто. Подождите минуту.")
		return
	}

	token, err := h.svc.CreateVerificationToken(r.Context(), userID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	link := h.cfg.PublicBaseURL + "/verify-email?token=" + token
	body := "<p>Здравствуйте, " + html.EscapeString(user.Username) + "!</p>" +
		"<p>Подтвердите email на «Намылимся».</p>" +
		"<p><a href=\"" + html.EscapeString(link) + "\">Подтвердить email</a></p>" +
		"<p>Ссылка действует 24 часа.</p>"
	_ = h.mailer.Send(r.Context(), email.Message{
		To:      user.Email,
		Subject: "Подтвердите email — Намылимся",
		Body:    body,
	})
	httpx.OK(w, map[string]string{"message": "Письмо отправлено"})
}

// ─── Password reset ────────────────────────────────────────────────

type passwordResetRequest struct {
	Email string `json:"email"`
}

// RequestPasswordReset — POST /auth/password-reset-request.
// Принимает email. Всегда 200 (enumeration protection).
func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req passwordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if req.Email == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "email is required")
		return
	}

	// mailer передаётся из main.go через cfg (или поле в AuthHandler)
	// Пока — заглушка, будем прокидывать через поле.
	_ = h.svc.RequestPasswordReset(r.Context(), req.Email, h.mailer)

	httpx.OK(w, map[string]string{
		"message": "Если email зарегистрирован, на него отправлен код.",
	})
}

type passwordResetConfirmRequest struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

// ResetPassword — POST /auth/password-reset.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req passwordResetConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}
	if req.Email == "" || req.Code == "" || req.NewPassword == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "email, code, new_password required")
		return
	}

	err := h.svc.ResetPassword(r.Context(), req.Email, req.Code, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidResetCode):
			httpx.Error(w, http.StatusBadRequest, "bad_request", "Неверный или истёкший код")
		case errors.Is(err, services.ErrTooManyAttempts):
			httpx.Error(w, http.StatusTooManyRequests, "rate_limit", "Слишком много попыток. Подождите 15 минут.")
		case errors.Is(err, services.ErrPasswordTooShort),
			errors.Is(err, services.ErrPasswordTooLong):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}

	httpx.OK(w, map[string]string{"message": "Пароль изменён. Войдите с новым паролем."})
}
