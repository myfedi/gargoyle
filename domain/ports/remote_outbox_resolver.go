package ports

import (
	"context"
	"encoding/json"

	"github.com/myfedi/gargoyle/domain/models"
)

// RemoteOutboxResolver resolves platform-compatible outbox pages when a remote
// ActivityPub outbox advertises content but does not expose collection pages.
type RemoteOutboxResolver interface {
	ResolveOutboxPage(ctx context.Context, signer models.Account, pageURI, expectedActor string) ([]json.RawMessage, string, error)
}
