package handlers

import (
	"encoding/json"
	"io"
	"net/http"
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
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	httpx.OK(w, updated)
}

func (h *UserHandler) GetPublic(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid user id")
		return
	}

	profile, err := h.svc.GetPublicProfile(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}

	// Скрываем возраст, если show_age = false
	resp := map[string]any{
		"id":         profile.ID,
		"username":   profile.Username,
		"gender":     profile.Gender,
		"city":       profile.City,
		"bio":        profile.Bio,
		"hobbies":    profile.Hobbies,
		"avatar_url": profile.AvatarUrl,
		"created_at": profile.CreatedAt,
	}
	if profile.ShowAge && profile.BirthDate.Valid {
		resp["age"] = calculateAge(profile.BirthDate.Time)
	}

	httpx.OK(w, resp)
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

	// Ограничиваем размер тела запроса (5 MB)
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "file too large or invalid form")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "missing avatar field")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	url, err := h.svc.UploadAvatar(r.Context(), userID, file, contentType, header.Size)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	httpx.OK(w, map[string]string{"avatar_url": url})
}

var _ = io.Discard // placeholder на случай, если io не нужен — удалите, если компилятор ругается на unused
