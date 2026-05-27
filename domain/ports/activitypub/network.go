package activitypub

import (
	"context"
	"net/url"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

// RemoteActorDocument contains the remote actor fields required by use cases
// and HTTP-signature verification. It intentionally models only the subset the
// application needs instead of leaking an infrastructure JSON shape.
type RemoteActorDocument struct {
	Inbox     string
	PublicKey RemoteActorPublicKey
}

type RemoteActorPublicKey struct {
	ID           string
	Owner        string
	PublicKeyPem string
}

// ActorFetcher resolves remote ActivityPub actors. Implementations usually use
// HTTP, but use cases only depend on this port.
type ActorFetcher interface {
	FetchActor(ctx context.Context, actor string, signer *models.Account) (*RemoteActorDocument, error)
}

// ActivityDeliverer delivers a signed ActivityPub payload to a remote inbox.
// Delivery is intentionally outside database transactions and may retry/fail
// independently from committed local state.
type ActivityDeliverer interface {
	Deliver(ctx context.Context, body []byte, inbox string, account models.Account)
}

// SignatureVerificationInput is the infrastructure-neutral request data needed
// to verify an inbound HTTP signature.
type SignatureVerificationInput struct {
	Method       string
	URL          *url.URL
	Host         string
	Headers      map[string]string
	Body         []byte
	Actor        string
	LocalAccount *models.Account
	Required     bool
}

// SignatureVerifier validates inbound ActivityPub HTTP signatures.
type SignatureVerifier interface {
	VerifyInbound(ctx context.Context, input SignatureVerificationInput) *domainerrors.DomainError
}
