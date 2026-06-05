package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type accountResolver struct {
	accountsRepo       repos.AccountsRepo
	remoteAccountsRepo repos.RemoteAccountsRepository
	domainBlocksRepo   repos.DomainBlocksRepository
	remoteResolver     RemoteAccountResolver
}

func accountResolverFromStatuses(cfg StatusesConfig) accountResolver {
	return accountResolver{accountsRepo: cfg.AccountsRepo, remoteAccountsRepo: cfg.RemoteAccountsRepo, domainBlocksRepo: cfg.DomainBlocksRepo, remoteResolver: cfg.RemoteResolver}
}

func accountResolverFromInteractions(cfg InteractionsConfig) accountResolver {
	return accountResolver{accountsRepo: cfg.AccountsRepo, remoteAccountsRepo: cfg.RemoteAccountsRepo, domainBlocksRepo: cfg.DomainBlocksRepo, remoteResolver: cfg.RemoteResolver}
}

func accountResolverFromNotifications(cfg NotificationsConfig) accountResolver {
	return accountResolver{accountsRepo: cfg.AccountsRepo, remoteAccountsRepo: cfg.RemoteAccountsRepo, domainBlocksRepo: cfg.DomainBlocksRepo, remoteResolver: cfg.RemoteResolver}
}

func (r accountResolver) getAccount(ctx context.Context, localAccount *models.Account, accountID string) (*models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	if accountID == localAccount.ID {
		return localAccount, nil
	}
	actor, err := RemoteActorFromAccountID(accountID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	remote, err := r.remoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, actor)
	if err == nil {
		if derr := r.ensureRemoteDomainAllowed(ctx, remote.URI); derr != nil {
			return nil, derr
		}
		return remote, nil
	}
	remote, err = r.resolveAndCacheRemoteAccount(ctx, actor, localAccount)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrNotFound, err)
	}
	if derr := r.ensureRemoteDomainAllowed(ctx, remote.URI); derr != nil {
		return nil, derr
	}
	return remote, nil
}

func (r accountResolver) resolveAndCacheRemoteAccount(ctx context.Context, query string, signer *models.Account) (*models.Account, error) {
	if derr := r.ensureRemoteDomainAllowed(ctx, query); derr != nil {
		return nil, derr
	}
	remote, err := r.remoteResolver.ResolveAccount(ctx, query, signer)
	if err != nil {
		return nil, err
	}
	if derr := r.ensureRemoteDomainAllowed(ctx, remote.URI); derr != nil {
		return nil, derr
	}
	return r.remoteAccountsRepo.UpsertRemoteAccount(ctx, nil, *remote)
}

func (r accountResolver) ensureRemoteDomainAllowed(ctx context.Context, actorOrURL string) *domainerrors.DomainError {
	return ensureRemoteDomainAllowed(ctx, r.domainBlocksRepo, actorOrURL)
}

func (u Statuses) resolveAndCacheRemoteAccount(ctx context.Context, query string, signer *models.Account) (*models.Account, error) {
	return accountResolverFromStatuses(u.deps).resolveAndCacheRemoteAccount(ctx, query, signer)
}

func (u Interactions) getAccount(ctx context.Context, localAccount *models.Account, accountID string) (*models.Account, *domainerrors.DomainError) {
	return accountResolverFromInteractions(u.deps).getAccount(ctx, localAccount, accountID)
}

func (u Notifications) resolveAndCacheRemoteAccount(ctx context.Context, query string, signer *models.Account) (*models.Account, error) {
	return accountResolverFromNotifications(u.deps).resolveAndCacheRemoteAccount(ctx, query, signer)
}
