package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	basePath string
	baseURL  string
}

func NewLocalStorage(basePath, baseURL string) (*LocalStorage, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir storage: %w", err)
	}
	return &LocalStorage{basePath: basePath, baseURL: baseURL}, nil
}

func (s *LocalStorage) Upload(ctx context.Context, key string, content io.Reader, contentType string, size int64) (string, error) {
	fullPath := filepath.Join(s.basePath, key)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", err
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, content); err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s", s.baseURL, key), nil
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	fullPath := filepath.Join(s.basePath, key)
	return os.Remove(fullPath)
}
