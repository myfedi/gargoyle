package adapters

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

type LocalMediaStorage struct{ dir string }

func NewLocalMediaStorage(dir string) *LocalMediaStorage { return &LocalMediaStorage{dir: dir} }

func (s *LocalMediaStorage) SaveMedia(ctx context.Context, id string, fileName string, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return "", err
	}
	ext := filepath.Ext(fileName)
	if strings.Contains(ext, string(filepath.Separator)) {
		ext = ""
	}
	name := id + ext
	path := filepath.Join(s.dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return name, nil
}

func (s *LocalMediaStorage) ReadMedia(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(s.dir, filepath.Base(path)))
}

func (s *LocalMediaStorage) DeleteMedia(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	err := os.Remove(filepath.Join(s.dir, filepath.Base(path)))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
