package ports

import "context"

type StoredMedia struct {
	Path string
	Data []byte
}

type MediaStorage interface {
	SaveMedia(ctx context.Context, id, fileName string, data []byte) (string, error)
	ReadMedia(ctx context.Context, path string) ([]byte, error)
	DeleteMedia(ctx context.Context, path string) error
}
