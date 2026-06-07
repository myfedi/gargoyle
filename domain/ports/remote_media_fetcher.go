package ports

import "context"

type FetchedRemoteMedia struct {
	Data        []byte
	ContentType string
	FileName    string
}

type RemoteMediaFetcher interface {
	FetchMedia(ctx context.Context, mediaURL string, maxBytes int64) (FetchedRemoteMedia, error)
}
