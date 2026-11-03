package services

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type UserService struct {
	db      *dbgen.Queries
	storage storage.FileStorage
}

func NewUserService(db *dbgen.Queries, s storage.FileStorage) *UserService {
	return &UserService{db: db, storage: s}
}

type UpdateProfileInput struct {
	Username  *string
	Gender    *string
	BirthDate *string
	City      *string
	Bio       *string
	Hobbies   []string
	Telegram  *string
	VK        *string
	ShowAge   *bool
}

func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, input UpdateProfileInput) (*dbgen.User, error) {
	// Получаем текущего пользователя, чтобы «мерджить» частичные апдейты
	current, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	params := dbgen.UpdateUserParams{
		ID:        userID,
		Username:  current.Username, // default — текущее значение
		Gender:    current.Gender,
		City:      current.City,
		Bio:       current.Bio,
		Hobbies:   current.Hobbies,
		Telegram:  current.Telegram,
		Vk:        current.Vk,
		AvatarUrl: current.AvatarUrl,
		ShowAge:   current.ShowAge,
	}

	// birth_date: если не передан — берём текущее
	var birthDate pgtype.Date
	if current.BirthDate.Valid {
		birthDate = current.BirthDate
	}
	if input.BirthDate != nil {
		if *input.BirthDate == "" {
			birthDate = pgtype.Date{Valid: false} // очистить
		} else {
			t, err := time.Parse("2006-01-02", *input.BirthDate)
			if err != nil {
				return nil, fmt.Errorf("invalid birth_date: %w", err)
			}
			birthDate = pgtype.Date{Time: t, Valid: true}
		}
	}
	params.BirthDate = birthDate

	if input.Username != nil {
		params.Username = *input.Username
	}
	if input.Gender != nil {
		params.Gender = input.Gender
	}
	if input.City != nil {
		params.City = input.City
	}
	if input.Bio != nil {
		params.Bio = input.Bio
	}
	if input.Hobbies != nil {
		params.Hobbies = input.Hobbies
	}
	if input.Telegram != nil {
		params.Telegram = input.Telegram
	}
	if input.VK != nil {
		params.Vk = input.VK
	}
	if input.ShowAge != nil {
		params.ShowAge = *input.ShowAge
	}

	updated, err := s.db.UpdateUser(ctx, params)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (s *UserService) GetPublicProfile(ctx context.Context, userID uuid.UUID) (*dbgen.GetPublicUserRow, error) {
	row, err := s.db.GetPublicUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *UserService) UploadAvatar(ctx context.Context, userID uuid.UUID, content io.Reader, contentType string, size int64) (string, error) {
	key := fmt.Sprintf("avatars/%s_%d", userID.String(), time.Now().Unix())
	url, err := s.storage.Upload(ctx, key, content, contentType, size)
	if err != nil {
		return "", err
	}
	// Сохранить URL в users.avatar_url
	current, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	params := dbgen.UpdateUserParams{
		ID:        userID,
		Username:  current.Username,
		Gender:    current.Gender,
		BirthDate: current.BirthDate,
		City:      current.City,
		Bio:       current.Bio,
		Hobbies:   current.Hobbies,
		Telegram:  current.Telegram,
		Vk:        current.Vk,
		AvatarUrl: &url,
		ShowAge:   current.ShowAge,
	}
	if _, err := s.db.UpdateUser(ctx, params); err != nil {
		return "", err
	}
	return url, nil
}
