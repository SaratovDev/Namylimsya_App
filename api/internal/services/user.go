package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/SaratovDev/Namylimsya_App/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

var (
	ErrInvalidAvatarURL        = errors.New("Недопустимый URL аватара")
	ErrInvalidAvatarFormat     = errors.New("Аватар должен быть изображением (JPEG, PNG, WebP или GIF)")
	ErrAvatarTooLarge          = errors.New("Аватар слишком большой (максимум 5 МБ)")
	ErrUsernameChangeForbidden = errors.New("Имя пользователя можно сменить только один раз")
	ErrUsernameTaken           = errors.New("Это имя уже занято")
	ErrUsernameTooShort        = errors.New("Имя пользователя должно быть не короче 3 символов")
	ErrUsernameTooLong         = errors.New("Ну ты разогнался. Давай не длиннее 30 символов")
	ErrUsernameInvalid         = errors.New("Имя может содержать только латинские буквы, цифры и _")
)

// Допустимые MIME-типы аватаров → расширение файла.
// Снифается сервером через http.DetectContentType, клиенту не доверяем.
var avatarExtByMIME = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

const maxAvatarBytes = 5 << 20 // 5 МБ

type UserService struct {
	db            *dbgen.Queries
	storage       storage.FileStorage
	avatarBaseURL string
}

func NewUserService(db *dbgen.Queries, s storage.FileStorage, avatarBaseURL string) *UserService {
	return &UserService{db: db, storage: s, avatarBaseURL: avatarBaseURL}
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
	AvatarUrl *string
}

func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, input UpdateProfileInput) (*dbgen.User, error) {
	current, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// ─── Username ───────────────────────────────────────────────────
	newUsername := current.Username
	if input.Username != nil {
		candidate := strings.TrimSpace(*input.Username)
		if candidate != current.Username {
			if len(candidate) < 3 {
				return nil, ErrUsernameTooShort
			}
			if len(candidate) > 30 {
				return nil, ErrUsernameTooLong
			}
			if !usernameRegex.MatchString(candidate) {
				return nil, ErrUsernameInvalid
			}
			if current.UsernameChangedAt != nil {
				return nil, ErrUsernameChangeForbidden
			}
			// Проверка на существующего пользователя — на случай, если UNIQUE
			// constraint не поймает (race). Основная защита — UNIQUE в БД.
			newUsername = candidate
		}
	}

	// ─── Birth date ─────────────────────────────────────────────────
	var birthDate pgtype.Date
	if current.BirthDate.Valid {
		birthDate = current.BirthDate
	}
	if input.BirthDate != nil {
		if *input.BirthDate == "" {
			birthDate = pgtype.Date{Valid: false}
		} else {
			t, err := time.Parse("2006-01-02", *input.BirthDate)
			if err != nil {
				return nil, fmt.Errorf("invalid birth_date: %w", err)
			}
			birthDate = pgtype.Date{Time: t, Valid: true}
		}
	}

	// ─── Other fields ────────────────────────────────────────────────
	params := dbgen.UpdateUserFullParams{
		ID:        userID,
		Username:  newUsername,
		Gender:    current.Gender,
		BirthDate: birthDate,
		City:      current.City,
		Bio:       current.Bio,
		Hobbies:   current.Hobbies,
		Telegram:  current.Telegram,
		Vk:        current.Vk,
		AvatarUrl: current.AvatarUrl,
		ShowAge:   current.ShowAge,
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
	if input.AvatarUrl != nil {
		if *input.AvatarUrl == "" {
			params.AvatarUrl = nil
		} else {
			if !s.isAllowedAvatarURL(*input.AvatarUrl) {
				return nil, ErrInvalidAvatarURL
			}
			params.AvatarUrl = input.AvatarUrl
		}
	}

	updated, err := s.db.UpdateUserFull(ctx, params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}

	// ─── Username history ───────────────────────────────────────────
	// Пишем старый ник в историю, только если он реально изменился.
	if newUsername != current.Username {
		if err := s.db.InsertUsernameHistory(ctx, dbgen.InsertUsernameHistoryParams{
			UserID:      userID,
			OldUsername: current.Username,
		}); err != nil {
			// Не критично — обновление прошло, история не записалась.
			log.Warn().Err(err).Str("user_id", userID.String()).
				Msg("failed to insert username history")
		}
	}

	return &updated, nil
}

// UploadAvatar валидирует содержимое серверным снифом (http.DetectContentType),
// а не по клиентскому Content-Type. Только image/jpeg, image/png, image/webp,
// image/gif — с соответствующим расширением в имени файла. Без этого файл
// без расширения и с валидным «клиентским» Content-Type: text/html отдавался
// бы Go-http.FileServer как HTML на origin API → stored XSS → кража refresh.
func (s *UserService) UploadAvatar(ctx context.Context, userID uuid.UUID, content io.Reader) (string, error) {
	// 1. Читаем с лимитом (max+1 байт, чтобы понять «слишком большой»).
	data, err := io.ReadAll(io.LimitReader(content, maxAvatarBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxAvatarBytes {
		return "", ErrAvatarTooLarge
	}
	if len(data) == 0 {
		return "", ErrInvalidAvatarFormat
	}

	// 2. Сниф контента: 512 байт достаточно для http.DetectContentType.
	detected := http.DetectContentType(data)
	ext, ok := avatarExtByMIME[detected]
	if !ok {
		return "", ErrInvalidAvatarFormat
	}

	// 3. Ключ с расширением: FileServer будет отдавать с правильным MIME.
	key := fmt.Sprintf("avatars/%s_%d%s", userID.String(), time.Now().Unix(), ext)

	url, err := s.storage.Upload(ctx, key, bytes.NewReader(data), detected, int64(len(data)))
	if err != nil {
		return "", err
	}

	// 4. Сохранить URL в users.avatar_url.
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

// isAllowedAvatarURL ограничивает avatar_url: либо наш пресет (/avatars/…),
// либо наш загруженный файл (avatarBaseURL + /static/avatars/…). Всё
// остальное (внешние домены, javascript:, data:) отклоняется — иначе
// пользователь может подсунуть другому ссылку на чужой трекер/скрипт в
// профиле/чате.
func (s *UserService) isAllowedAvatarURL(v string) bool {
	if v == "" {
		return true
	}
	if strings.HasPrefix(v, "/avatars/") {
		return true
	}
	if s.avatarBaseURL != "" && strings.HasPrefix(v, s.avatarBaseURL+"/static/avatars/") {
		return true
	}
	return false
}

// GetPublicProfileFull — расширенный публичный профиль: базовые поля +
// счётчики организованных/посещённых + последние 10 событий в каждом.
// Возраст — только если show_age; иначе nil (фронт не покажет).
func (s *UserService) GetPublicProfileFull(
	ctx context.Context,
	userID uuid.UUID,
	limit int32,
) (*dbgen.GetPublicUserFullRow, []dbgen.ListUserOrganizedCompletedRow, []dbgen.ListUserAttendedCompletedRow, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	user, err := s.db.GetPublicUserFull(ctx, userID)
	if err != nil {
		return nil, nil, nil, err
	}

	organized, err := s.db.ListUserOrganizedCompleted(ctx, dbgen.ListUserOrganizedCompletedParams{
		OrganizerID: userID,
		Limit:       limit,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	attended, err := s.db.ListUserAttendedCompleted(ctx, dbgen.ListUserAttendedCompletedParams{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	return &user, organized, attended, nil
}

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// GetUsernameHistory — история смены никнеймов (для публичного профиля
// и страницы настроек).
func (s *UserService) GetUsernameHistory(ctx context.Context, userID uuid.UUID) ([]dbgen.GetUsernameHistoryRow, error) {
	return s.db.GetUsernameHistory(ctx, userID)
}
