package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

func (u Accounts) cacheRemoteOutbox(ctx context.Context, localAccount *models.Account, remote models.Account) *domainerrors.DomainError {
	if remote.OutboxURI == nil || *remote.OutboxURI == "" || localAccount == nil {
		return nil
	}
	if err := u.deps.HydrateRemoteObjectUC.CacheRemoteOutbox(ctx, *localAccount, *remote.OutboxURI, remote.URI); err != nil {
		return nil
	}
	return nil
}
