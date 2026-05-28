package mastodon

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

// SearchAccounts resolves a remote account from an acct URI or actor URL. It
// returns Mastodon-shaped account data via the handler while keeping WebFinger
// and HTTP behind RemoteAccountResolver.
func (u UseCase) SearchAccounts(ctx context.Context, account *models.Account, query string) ([]models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []models.Account{}, nil
	}
	remote, err := u.cfg.RemoteResolver.ResolveAccount(ctx, query, account)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
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
	remote, err := u.cfg.RemoteResolver.ResolveAccount(ctx, actor, localAccount)
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
	remote, err := u.cfg.RemoteResolver.ResolveAccount(ctx, actor, localAccount)
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
	raw, err := json.Marshal(map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": localAccount.URI + "/undos/" + undoID, "type": "Undo", "actor": localAccount.URI, "object": map[string]any{"type": "Follow", "actor": localAccount.URI, "object": remote.URI}})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &FollowAccountResult{Account: *localAccount, RawJSON: raw, Inbox: remote.InboxURI}, nil
}
