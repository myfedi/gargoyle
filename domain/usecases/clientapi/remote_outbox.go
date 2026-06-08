package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

func (u Accounts) cacheRemoteOutboxUntil(ctx context.Context, localAccount *models.Account, remote models.Account, enough func() (bool, error)) *domainerrors.DomainError {
	if remote.OutboxURI == nil || *remote.OutboxURI == "" || localAccount == nil {
		return nil
	}
	page := *remote.OutboxURI
	seen := map[string]bool{}
	for page != "" && !seen[page] {
		ok, err := enough()
		if err != nil {
			return domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		if ok {
			return nil
		}
		seen[page] = true
		next, err := u.deps.HydrateRemoteObjectUC.CacheRemoteOutboxPage(ctx, *localAccount, page, remote.URI, enough)
		if err != nil {
			return nil
		}
		page = next
	}
	return nil
}
