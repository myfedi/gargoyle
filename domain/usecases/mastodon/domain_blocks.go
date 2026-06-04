package mastodon

import (
	"context"
	"net/url"
	"strings"

	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

func (u UseCase) ensureRemoteDomainAllowed(ctx context.Context, actorOrURL string) *domainerrors.DomainError {
	domain := domainFromURL(actorOrURL)
	if domain == "" {
		return nil
	}
	blocked, err := u.cfg.DomainBlocksRepo.DomainIsSuspended(ctx, nil, domain)
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if blocked {
		return domainerrors.New(domainerrors.ErrUnauthorized, "remote domain is suspended")
	}
	return nil
}

func domainFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
