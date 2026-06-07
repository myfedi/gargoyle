package clientapi

import (
	"context"
	"io"
	"net/http"
	"path/filepath"

	"github.com/myfedi/gargoyle/domain/ports"
)

type RemoteMediaFetcher struct {
	client     *http.Client
	exceptions []RemoteURLException
}

func NewRemoteMediaFetcher(client *http.Client, exceptions []RemoteURLException) RemoteMediaFetcher {
	return RemoteMediaFetcher{client: publicOnlyHTTPClient(client, exceptions), exceptions: exceptions}
}

func (f RemoteMediaFetcher) FetchMedia(ctx context.Context, mediaURL string, maxBytes int64) (ports.FetchedRemoteMedia, error) {
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}
	if err := validateRemoteURL(ctx, mediaURL, f.exceptions); err != nil {
		return ports.FetchedRemoteMedia{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return ports.FetchedRemoteMedia{}, err
	}
	req.Header.Set("Accept", "image/*, video/*, audio/*")
	resp, err := f.client.Do(req)
	if err != nil {
		return ports.FetchedRemoteMedia{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ports.FetchedRemoteMedia{}, httpError(resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return ports.FetchedRemoteMedia{}, err
	}
	if int64(len(data)) > maxBytes {
		return ports.FetchedRemoteMedia{}, httpError(http.StatusRequestEntityTooLarge)
	}
	return ports.FetchedRemoteMedia{Data: data, ContentType: resp.Header.Get("Content-Type"), FileName: filepath.Base(req.URL.Path)}, nil
}
