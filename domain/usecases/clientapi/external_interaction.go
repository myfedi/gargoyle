package clientapi

import (
	"context"
	"net/url"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

const ExternalInteractionTypeAccount = "account"

// ExternalInteraction resolves a user-supplied remote Fediverse URI into a
// local, authenticated interaction target. It owns the actor-vs-object decision
// surface so HTTP handlers and browser routes stay transport-only.
type ExternalInteraction struct {
	deps     ExternalInteractionConfig
	resolver accountResolver
}

type ExternalInteractionResult struct {
	Type    string
	Account *models.Account
}

func (u ExternalInteraction) Resolve(ctx context.Context, localAccount *models.Account, uri string) (*ExternalInteractionResult, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "remote interaction URI is required")
	}
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "remote interaction URI must be an HTTP or HTTPS URL")
	}

	remote, err := u.resolver.resolveAndCacheRemoteAccount(ctx, uri, localAccount)
	if err != nil {
		return nil, remoteResolveError(err)
	}
	if derr := u.resolver.ensureRemoteDomainAllowed(ctx, remote.URI); derr != nil {
		return nil, derr
	}
	return &ExternalInteractionResult{Type: ExternalInteractionTypeAccount, Account: remote}, nil
}
