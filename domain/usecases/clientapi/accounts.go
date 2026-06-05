package clientapi

import (
	"context"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

// SearchAccounts returns local and cached remote accounts without network I/O.
// Client applications use this endpoint for debounced typeahead, so it must remain fast.
func (u Accounts) SearchAccounts(ctx context.Context, account *models.Account, query string, limit int) ([]models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	query = strings.TrimSpace(strings.TrimPrefix(query, "@"))
	if query == "" {
		return []models.Account{}, nil
	}
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	locals, err := u.deps.AccountsRepo.SearchLocalAccounts(ctx, nil, query, limit)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	remaining := limit - len(locals)
	if remaining <= 0 {
		return locals[:limit], nil
	}
	remote, err := u.deps.RemoteAccountsRepo.SearchRemoteAccounts(ctx, nil, query, remaining)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return append(locals, u.filterBlockedAccounts(ctx, remote)...), nil
}

// ResolveAccountSearch performs the explicit remote resolution path used by
// /api/v2/search?type=accounts&resolve=true when a user confirms a full acct or URL.
func (u Accounts) ResolveAccountSearch(ctx context.Context, account *models.Account, query string) ([]models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []models.Account{}, nil
	}
	remote, err := u.resolveAndCacheRemoteAccount(ctx, query, account)
	if err != nil {
		return nil, remoteResolveError(err)
	}
	if derr := u.ensureRemoteDomainAllowed(ctx, remote.URI); derr != nil {
		return nil, derr
	}
	return []models.Account{*remote}, nil
}

// FollowAccount creates and returns a signed Follow activity for delivery by the
// HTTP adapter after the local following state is committed.
func (u Accounts) FollowAccount(ctx context.Context, localAccount *models.Account, accountID string) (*FollowAccountResult, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	actor, err := RemoteActorFromAccountID(accountID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	remote, err := u.resolveAndCacheRemoteAccount(ctx, actor, localAccount)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	followID, err := u.deps.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res, derr := u.deps.CreateFollowingUC.CreateFollowing(ctx, apUsecases.CreateFollowingInput{Username: localAccount.Username, Actor: remote.URI, Inbox: remote.InboxURI, FollowID: followID})
	if derr != nil {
		return nil, derr
	}
	return &FollowAccountResult{Account: res.Account, RawJSON: res.RawJSON, Inbox: res.Inbox}, nil
}

// UnfollowAccount removes a following relationship and returns an Undo Follow
// activity for delivery to the remote actor's inbox. The Undo object is the
// Follow shape most ActivityPub servers understand for undoing follows.
func (u Accounts) UnfollowAccount(ctx context.Context, localAccount *models.Account, accountID string) (*FollowAccountResult, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	actor, err := RemoteActorFromAccountID(accountID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	remote, err := u.resolveAndCacheRemoteAccount(ctx, actor, localAccount)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	undoID, err := u.deps.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res, derr := u.deps.UndoFollowingUC.UndoFollowing(ctx, apUsecases.UndoFollowingInput{Username: localAccount.Username, Actor: remote.URI, Inbox: remote.InboxURI, UndoID: undoID})
	if derr != nil {
		return nil, derr
	}
	return &FollowAccountResult{Account: res.Account, RawJSON: res.RawJSON, Inbox: res.Inbox}, nil
}

func remoteResolveError(err error) *domainerrors.DomainError {
	message := err.Error()
	if strings.Contains(message, "actor fetch failed with status 401") || strings.Contains(message, "actor fetch failed with status 403") {
		return domainerrors.New(domainerrors.ErrBadRequest, "Remote server rejected the lookup. It may require signed requests and could not verify this server's public ActivityPub identity.")
	}
	if strings.Contains(message, "webfinger failed with status 404") {
		return domainerrors.New(domainerrors.ErrNotFound, "No remote account found.")
	}
	return domainerrors.New(domainerrors.ErrBadRequest, "Could not look up that remote account.")
}

func (u Accounts) resolveAndCacheRemoteAccount(ctx context.Context, query string, signer *models.Account) (*models.Account, error) {
	if derr := u.ensureRemoteDomainAllowed(ctx, query); derr != nil {
		return nil, derr
	}
	remote, err := u.deps.RemoteResolver.ResolveAccount(ctx, query, signer)
	if err != nil {
		return nil, err
	}
	if derr := u.ensureRemoteDomainAllowed(ctx, remote.URI); derr != nil {
		return nil, derr
	}
	return u.deps.RemoteAccountsRepo.UpsertRemoteAccount(ctx, nil, *remote)
}

func (u Accounts) filterBlockedAccounts(ctx context.Context, accounts []models.Account) []models.Account {
	res := make([]models.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.Domain == nil || *account.Domain == "" {
			res = append(res, account)
			continue
		}
		blocked, err := u.deps.DomainBlocksRepo.DomainIsSuspended(ctx, nil, *account.Domain)
		if err == nil && !blocked {
			res = append(res, account)
		}
	}
	return res
}
