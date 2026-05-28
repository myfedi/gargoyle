package ports

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
)

// RemoteObjectFetcher retrieves remote ActivityPub objects for asynchronous
// hydration jobs such as missing reply parents.
type RemoteObjectFetcher interface {
	FetchObject(ctx context.Context, objectURI string, signer *models.Account) error
}
