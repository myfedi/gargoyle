package mastodon

import (
	"context"
	"io"
	"net/http"

	"github.com/myfedi/gargoyle/domain/models"
)

// RemoteObjectFetcher retrieves remote ActivityPub objects for fetch jobs. It
// validates and signs fetches, then returns the bounded response body for the
// domain hydration use case to parse and persist.
type RemoteObjectFetcher struct {
	client     *http.Client
	exceptions []RemoteURLException
}

func NewRemoteObjectFetcher(client *http.Client, exceptions []RemoteURLException) RemoteObjectFetcher {
	return RemoteObjectFetcher{client: publicOnlyHTTPClient(client, exceptions), exceptions: exceptions}
}

func (f RemoteObjectFetcher) FetchObject(ctx context.Context, objectURI string, signer *models.Account) ([]byte, error) {
	if err := validateRemoteURL(ctx, objectURI, f.exceptions); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, objectURI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/activity+json")
	if signer != nil {
		signFederatedGet(req, *signer)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpError(resp.StatusCode)
	}
	if readErr != nil {
		return nil, readErr
	}
	return body, nil
}

type httpError int

func (e httpError) Error() string { return http.StatusText(int(e)) }
