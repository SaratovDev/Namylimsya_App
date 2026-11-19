package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/httpx"
	"github.com/SaratovDev/Namylimsya_App/api/internal/middleware"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type UserHandler struct {
	svc *services.UserService
}

func NewUserHandler(svc *services.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

type updateProfileRequest struct {
	Username  *string  `json:"username,omitempty"`
	Gender    *string  `json:"gender,omitempty"`
	BirthDate *string  `json:"birth_date,omitempty"`
	City      *string  `json:"city,omitempty"`
	Bio       *string  `json:"bio,omitempty"`
	Hobbies   []string `json:"hobbies,omitempty"`
	Telegram  *string  `json:"telegram,omitempty"`
	VK        *string  `json:"vk,omitempty"`
	ShowAge   *bool    `json:"show_age,omitempty"`
	AvatarUrl *string  `json:"avatar_url,omitempty"`
}

func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	updated, err := h.svc.UpdateProfile(r.Context(), userID, services.UpdateProfileInput{
		Username:  req.Username,
		Gender:    req.Gender,
		BirthDate: req.BirthDate,
		City:      req.City,
		Bio:       req.Bio,
		Hobbies:   req.Hobbies,
		Telegram:  req.Telegram,
		VK:        req.VK,
		ShowAge:   req.ShowAge,
		AvatarUrl: req.AvatarUrl,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUsernameChangeForbidden),
			errors.Is(err, services.ErrUsernameTooShort),
			errors.Is(err, services.ErrUsernameTooLong),
			errors.Is(err, services.ErrUsernameInvalid),
			errors.Is(err, services.ErrUsernameTaken),
			errors.Is(err, services.ErrInvalidAvatarURL):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.InternalError(w, err)
		}
		return
	}

	httpx.OK(w, toPublicUser(updated))
}

// GetPublic — публичный профиль пользователя. Без авторизации.
// Отдаёт: базовые поля + роль/бан + звание (derived) + N дней на платформе
// + последние 10 организованных/посещённых завершённых событий.
func (h *UserHandler) GetPublic(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid user id")
		return
	}

	// limit — сколько событий в каждом списке. По умолчанию 10, максимум 50.
	limit := int32(10)
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 50 {
			limit = int32(v)
		}
	}

	profile, organized, attended, err := h.svc.GetPublicProfileFull(r.Context(), id, limit)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}

	resp := map[string]any{
		"id":               profile.ID,
		"username":         profile.Username,
		"gender":           profile.Gender,
		"city":             profile.City,
		"bio":              profile.Bio,
		"hobbies":          profile.Hobbies,
		"avatar_url":       profile.AvatarUrl,
		"role":             profile.Role,
		"is_banned":        profile.IsBanned,
		"created_at":       profile.CreatedAt,
		"organized_count":  profile.OrganizedCount,
		"attended_count":   profile.AttendedCount,
		"title":            computeTitle(profile.OrganizedCount + profile.AttendedCount),
		"days_on_platform": int(time.Since(profile.CreatedAt).Hours() / 24),
	}
	if profile.ShowAge && profile.BirthDate.Valid {
		resp["age"] = calculateAge(profile.BirthDate.Time)
	}

	// Списки событий.
	organizedOut := make([]map[string]any, 0, len(organized))
	for _, e := range organized {
		organizedOut = append(organizedOut, map[string]any{
			"id":                 e.ID,
			"title":              e.Title,
			"category":           e.Category,
			"starts_at":          e.StartsAt,
			"city":               e.City,
			"participants_count": e.ParticipantsCount,
		})
	}
	attendedOut := make([]map[string]any, 0, len(attended))
	for _, e := range attended {
		attendedOut = append(attendedOut, map[string]any{
			"id":               e.ID,
			"title":            e.Title,
			"category":         e.Category,
			"starts_at":        e.StartsAt,
			"city":             e.City,
			"organizer_name":   e.OrganizerName,
			"organizer_avatar": e.OrganizerAvatar,
		})
	}

	resp["organized_events"] = organizedOut
	resp["attended_events"] = attendedOut

	httpx.OK(w, resp)
}

// computeTitle — derived-звание по сумме organized+attended.
// Система наград (badges) позже может переопределить, но пока — это.
func computeTitle(total int64) string {
	switch {
	case total == 0:
		return "Новичок"
	case total <= 2:
		return "Первопроходец"
	case total <= 5:
		return "Активист"
	case total <= 10:
		return "Опытный"
	case total <= 20:
		return "Ветеран"
	default:
		return "Легенда"
	}
}

func calculateAge(birth time.Time) int {
	now := time.Now()
	age := now.Year() - birth.Year()
	if now.YearDay() < birth.YearDay() {
		age--
	}
	return age
}

func (h *UserHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	// Ограничиваем размер тела запроса (5 MB).
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "file too large or invalid form")
		return
	}

	file, _, err := r.FormFile("avatar")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "missing avatar field")
		return
	}
	defer file.Close()

	url, err := h.svc.UploadAvatar(r.Context(), userID, file)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidAvatarFormat),
			errors.Is(err, services.ErrAvatarTooLarge):
			httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, "internal", "failed to upload avatar")
		}
		return
	}

	httpx.OK(w, map[string]string{"avatar_url": url})
}

var _ = io.Discard // placeholder на случай, если io не нужен — удалите, если компилятор ругается на unused

// GetUsernameHistory — публичная история смены ников для /users/[id].
func (h *UserHandler) GetUsernameHistory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid user id")
		return
	}
	rows, err := h.svc.GetUsernameHistory(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.OK(w, map[string]any{"history": rows})
}
