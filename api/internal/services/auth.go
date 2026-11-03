package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/auth"
	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrUserExists   = errors.New("user already exists")
	ErrInvalidCreds = errors.New("invalid credentials")
)

type AuthService struct {
	db *dbgen.Queries
	tm *auth.TokenManager
	sm *auth.SessionManager
}

func NewAuthService(db *dbgen.Queries, tm *auth.TokenManager, sm *auth.SessionManager) *AuthService {
	return &AuthService{db: db, tm: tm, sm: sm}
}

type RegisterInput struct {
	Email     string
	Password  string
	Username  string
	Gender    string
	BirthDate string
	City      string
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*dbgen.User, error) {
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

	user, err := s.db.CreateUser(ctx, dbgen.CreateUserParams{
		Email:        input.Email,
		PasswordHash: hash,
		Username:     input.Username,
		Gender:       gender,
		BirthDate:    birthDate,
		City:         city,
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
	user, err := s.db.GetUserByEmail(ctx, email)
	if err != nil {
		return "", "", nil, ErrInvalidCreds
	}

	if !auth.CheckPasswordHash(password, user.PasswordHash) {
		return "", "", nil, ErrInvalidCreds
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
	user, err := s.db.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return "", "", err
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
