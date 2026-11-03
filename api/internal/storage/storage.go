package storage

import (
	"context"
	"io"
)

type FileStorage interface {
	// Upload сохраняет файл, возвращает публичный URL
	Upload(ctx context.Context, key string, content io.Reader, contentType string, size int64) (string, error)
	// Delete удаляет файл по ключу
	Delete(ctx context.Context, key string) error
}
