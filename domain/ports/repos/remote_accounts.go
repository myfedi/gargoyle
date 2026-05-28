package repos

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

// RemoteAccountsRepository caches ActivityPub actor metadata for client API
// reads so followers/following/search do not need to fetch every actor live.
type RemoteAccountsRepository interface {
	UpsertRemoteAccount(ctx context.Context, tx *db.Tx, account models.Account) (*models.Account, error)
	GetRemoteAccountByURI(ctx context.Context, tx *db.Tx, uri string) (*models.Account, error)
}
