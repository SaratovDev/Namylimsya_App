package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/SaratovDev/Namylimsya_App/api/internal/httpx"
	"github.com/SaratovDev/Namylimsya_App/api/internal/middleware"
	"github.com/SaratovDev/Namylimsya_App/api/internal/services"
)

type AuthHandler struct {
	svc *services.AuthService
}

func NewAuthHandler(svc *services.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	Username  string `json:"username"`
	Gender    string `json:"gender,omitempty"`
	BirthDate string `json:"birth_date,omitempty"`
	City      string `json:"city,omitempty"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	// Базовая валидация (в будущем можно вынести в отдельный пакет)
	if req.Email == "" || req.Password == "" || req.Username == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "email, password and username are required")
		return
	}

	user, err := h.svc.Register(r.Context(), services.RegisterInput{
		Email:     req.Email,
		Password:  req.Password,
		Username:  req.Username,
		Gender:    req.Gender,
		BirthDate: req.BirthDate,
		City:      req.City,
	})

	if err != nil {
		if errors.Is(err, services.ErrUserExists) {
			httpx.Error(w, http.StatusConflict, "conflict", "user already exists")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to register")
		return
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
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid email or password")
		return
	}

	// В MVP refresh токен кладем в httpOnly cookie (безопаснее для веба)
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // В проде нужно true
		SameSite: http.SameSiteLaxMode,
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

	httpx.JSON(w, http.StatusOK, user)
}
