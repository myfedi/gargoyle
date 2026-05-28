package mastodon

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

// Relationships returns whether the authenticated local account follows each
// requested account ID. Remote account IDs encode actor URIs so callers can use
// Mastodon-compatible IDs without a remote account cache yet.
func (u UseCase) Relationships(ctx context.Context, localAccount *models.Account, ids []string) (map[string]bool, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	following, err := u.cfg.FollowsRepo.ListFollowing(ctx, nil, localAccount.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	byActor := map[string]bool{}
	for _, follow := range following {
		byActor[follow.RemoteActor] = true
	}
	res := map[string]bool{}
	for _, id := range ids {
		actor, err := RemoteActorFromAccountID(id)
		if err != nil {
			res[id] = false
			continue
		}
		res[id] = byActor[actor]
	}
	return res, nil
}

// FollowingAccounts resolves accepted outbound follows into displayable account
// data for Mastodon-compatible following lists.
func (u UseCase) FollowingAccounts(ctx context.Context, localAccount *models.Account, accountID string) ([]models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	if accountID != localAccount.ID {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "account not found")
	}
	following, err := u.cfg.FollowsRepo.ListFollowing(ctx, nil, localAccount.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.resolveFollowActors(ctx, localAccount, following)
}

// FollowerAccounts resolves accepted inbound followers into displayable account
// data for Mastodon-compatible follower lists.
func (u UseCase) FollowerAccounts(ctx context.Context, localAccount *models.Account, accountID string) ([]models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	if accountID != localAccount.ID {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "account not found")
	}
	followers, err := u.cfg.FollowsRepo.ListFollowers(ctx, nil, localAccount.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.resolveFollowActors(ctx, localAccount, followers)
}

func (u UseCase) resolveFollowActors(ctx context.Context, localAccount *models.Account, follows []models.Follow) ([]models.Account, *domainerrors.DomainError) {
	accounts := make([]models.Account, 0, len(follows))
	for _, follow := range follows {
		remote, err := u.cfg.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, follow.RemoteActor)
		if err != nil {
			remote, err = u.resolveAndCacheRemoteAccount(ctx, follow.RemoteActor, localAccount)
			if err != nil {
				continue
			}
		}
		accounts = append(accounts, *remote)
	}
	return accounts, nil
}
