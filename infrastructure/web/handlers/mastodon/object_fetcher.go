package mastodon

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
)

// RemoteObjectFetcher retrieves remote ActivityPub objects for fetch jobs. The
// current implementation validates and signs the request, which is enough to
// warm remote servers and verify accessibility; object persistence is added by
// the use case that knows the local timeline context.
type RemoteObjectFetcher struct {
	client     *http.Client
	exceptions []RemoteURLException
}

func NewRemoteObjectFetcher(client *http.Client, exceptions []RemoteURLException) RemoteObjectFetcher {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return RemoteObjectFetcher{client: client, exceptions: exceptions}
}

func (f RemoteObjectFetcher) FetchObject(ctx context.Context, objectURI string, signer *models.Account) error {
	if err := validateRemoteURL(ctx, objectURI, f.exceptions); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, objectURI, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/activity+json")
	if signer != nil {
		signFederatedGet(req, *signer)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpError(resp.StatusCode)
	}
	return nil
}

type httpError int

func (e httpError) Error() string { return http.StatusText(int(e)) }
