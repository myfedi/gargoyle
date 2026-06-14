package ports

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
)

// RemoteThreadResolver discovers replies for remote objects when generic
// ActivityPub replies collections are unavailable or incomplete.
type RemoteThreadResolver interface {
	ResolveReplies(ctx context.Context, objectURI string, signer *models.Account) ([]RemoteReply, error)
}

type RemoteReply struct {
	URI          string
	AttributedTo string
	Content      string
	PublishedAt  time.Time
	InReplyToURI string
	To           []string
	CC           []string
	Sensitive    bool
	SpoilerText  string
}
