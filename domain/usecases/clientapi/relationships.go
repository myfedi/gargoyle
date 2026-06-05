package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

// Relationships returns whether the authenticated local account follows each
// requested account ID. Remote account IDs encode actor URIs so callers can use
// client-compatible IDs without a remote account cache yet.
func (u Accounts) Relationships(ctx context.Context, localAccount *models.Account, ids []string) (map[string]Relationship, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	following, err := u.deps.FollowsRepo.ListFollowingIncludingPending(ctx, nil, localAccount.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	byActor := map[string]models.Follow{}
	for _, follow := range following {
		byActor[follow.RemoteActor] = follow
	}
	res := map[string]Relationship{}
	for _, id := range ids {
		rel := Relationship{ID: id}
		actor, err := RemoteActorFromAccountID(id)
		if err == nil {
			if follow, ok := byActor[actor]; ok {
				rel.Following = follow.AcceptedAt != nil
				rel.Requested = follow.AcceptedAt == nil
			}
		}
		res[id] = rel
	}
	return res, nil
}

// FollowingAccounts resolves accepted outbound follows into displayable account
// data for client-compatible following lists.
func (u Accounts) FollowingAccounts(ctx context.Context, localAccount *models.Account, accountID string) ([]models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	if accountID != localAccount.ID {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "account not found")
	}
	following, err := u.deps.FollowsRepo.ListFollowing(ctx, nil, localAccount.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.resolveFollowActors(ctx, localAccount, following)
}

// FollowerAccounts resolves accepted inbound followers into displayable account
// data for client-compatible follower lists.
func (u Accounts) FollowerAccounts(ctx context.Context, localAccount *models.Account, accountID string) ([]models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	if accountID != localAccount.ID {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "account not found")
	}
	followers, err := u.deps.FollowsRepo.ListFollowers(ctx, nil, localAccount.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.resolveFollowActors(ctx, localAccount, followers)
}

func (u Accounts) resolveFollowActors(ctx context.Context, localAccount *models.Account, follows []models.Follow) ([]models.Account, *domainerrors.DomainError) {
	accounts := make([]models.Account, 0, len(follows))
	for _, follow := range follows {
		remote, err := u.deps.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, follow.RemoteActor)
		if err != nil {
			remote, err = u.resolveAndCacheRemoteAccount(ctx, follow.RemoteActor, localAccount)
			if err != nil {
				continue
			}
		}
		if derr := u.ensureRemoteDomainAllowed(ctx, remote.URI); derr != nil {
			continue
		}
		accounts = append(accounts, *remote)
	}
	return accounts, nil
}
