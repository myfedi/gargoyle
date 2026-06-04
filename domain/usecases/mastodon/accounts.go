package mastodon

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

// SearchAccounts returns local and cached remote accounts without network I/O.
// Mastodon clients use this endpoint for debounced typeahead, so it must remain fast.
func (u UseCase) SearchAccounts(ctx context.Context, account *models.Account, query string, limit int) ([]models.Account, *domainerrors.DomainError) {
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
	locals, err := u.cfg.AccountsRepo.SearchLocalAccounts(ctx, nil, query, limit)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	remaining := limit - len(locals)
	if remaining <= 0 {
		return locals[:limit], nil
	}
	remote, err := u.cfg.RemoteAccountsRepo.SearchRemoteAccounts(ctx, nil, query, remaining)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return append(locals, u.filterBlockedAccounts(ctx, remote)...), nil
}

// ResolveAccountSearch performs the explicit remote resolution path used by
// /api/v2/search?type=accounts&resolve=true when a user confirms a full acct or URL.
func (u UseCase) ResolveAccountSearch(ctx context.Context, account *models.Account, query string) ([]models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []models.Account{}, nil
	}
	remote, err := u.resolveAndCacheRemoteAccount(ctx, query, account)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	if derr := u.ensureRemoteDomainAllowed(ctx, remote.URI); derr != nil {
		return nil, derr
	}
	return []models.Account{*remote}, nil
}

// FollowAccount creates and returns a signed Follow activity for delivery by the
// HTTP adapter after the local following state is committed.
func (u UseCase) FollowAccount(ctx context.Context, localAccount *models.Account, accountID string) (*FollowAccountResult, *domainerrors.DomainError) {
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
	followID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res, derr := u.cfg.CreateFollowingUC.CreateFollowing(ctx, apUsecases.CreateFollowingInput{Username: localAccount.Username, Actor: remote.URI, Inbox: remote.InboxURI, FollowID: followID})
	if derr != nil {
		return nil, derr
	}
	return &FollowAccountResult{Account: res.Account, RawJSON: res.RawJSON, Inbox: res.Inbox}, nil
}

// UnfollowAccount removes a following relationship and returns an Undo Follow
// activity for delivery to the remote actor's inbox. The Undo object is the
// Follow shape most ActivityPub servers understand for undoing follows.
func (u UseCase) UnfollowAccount(ctx context.Context, localAccount *models.Account, accountID string) (*FollowAccountResult, *domainerrors.DomainError) {
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
	if err := u.cfg.FollowsRepo.DeleteFollowingByActor(ctx, nil, localAccount.ID, remote.URI); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	undoID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	raw, err := json.Marshal(map[string]any{activityStreamsContextKey: activityStreamsContextURI, "id": localAccount.URI + "/undos/" + undoID, "type": "Undo", "actor": localAccount.URI, "object": map[string]any{"type": "Follow", "actor": localAccount.URI, "object": remote.URI}})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &FollowAccountResult{Account: *localAccount, RawJSON: raw, Inbox: remote.InboxURI}, nil
}

func (u UseCase) resolveAndCacheRemoteAccount(ctx context.Context, query string, signer *models.Account) (*models.Account, error) {
	if derr := u.ensureRemoteDomainAllowed(ctx, query); derr != nil {
		return nil, derr
	}
	remote, err := u.cfg.RemoteResolver.ResolveAccount(ctx, query, signer)
	if err != nil {
		return nil, err
	}
	if derr := u.ensureRemoteDomainAllowed(ctx, remote.URI); derr != nil {
		return nil, derr
	}
	return u.cfg.RemoteAccountsRepo.UpsertRemoteAccount(ctx, nil, *remote)
}

func (u UseCase) filterBlockedAccounts(ctx context.Context, accounts []models.Account) []models.Account {
	res := make([]models.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.Domain == nil || *account.Domain == "" {
			res = append(res, account)
			continue
		}
		blocked, err := u.cfg.DomainBlocksRepo.DomainIsSuspended(ctx, nil, *account.Domain)
		if err == nil && !blocked {
			res = append(res, account)
		}
	}
	return res
}
