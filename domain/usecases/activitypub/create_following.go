package activitypub

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// CreateFollowingInput contains the command data for following a remote actor.
type CreateFollowingInput struct {
	Username string
	Actor    string
	Inbox    string
	FollowID string
}

// CreateFollowingResult contains the committed Follow payload and optional inbox
// for infrastructure delivery after the transaction has completed.
type CreateFollowingResult struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

// GetLocalAccount exposes local account lookup for infrastructure that needs to
// sign remote actor discovery before executing the command.
func (u *CreateFollowingUseCase) GetLocalAccount(ctx context.Context, username string) (*models.Account, *domainerrors.DomainError) {
	return localAccount(ctx, u.cfg.AccountsRepo, username)
}

// CreateFollowing stores the outbound Follow activity and following row in one
// transaction. Remote delivery is represented in the result and performed by an adapter.
func (u *CreateFollowingUseCase) CreateFollowing(ctx context.Context, input CreateFollowingInput) (*CreateFollowingResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, input.Username)
	if derr != nil {
		return nil, derr
	}
	if input.Actor == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "actor is required")
	}
	if input.FollowID == "" {
		return nil, domainerrors.New(domainerrors.ErrInternal, "follow id is required")
	}
	if input.Inbox == "" && u.cfg.ActorFetcher != nil {
		actor, err := u.cfg.ActorFetcher.FetchActor(ctx, input.Actor, account)
		if err == nil && actor != nil {
			input.Inbox = actor.Inbox
		}
	}

	followActivity := map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": account.URI + "/follows/" + input.FollowID, "type": "Follow", "actor": account.URI, "object": input.Actor}
	raw, err := json.Marshal(followActivity)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	err = u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		stored, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionOutbox, Type: "Follow", Actor: account.URI, Object: input.Actor, RawJSON: string(raw)})
		if err != nil {
			return err
		}
		var inboxPtr *string
		if input.Inbox != "" {
			inboxPtr = &input.Inbox
		}
		_, err = u.cfg.FollowsRepo.CreateFollowing(ctx, &tx, repos.CreateFollowInput{LocalAccountID: account.ID, RemoteActor: input.Actor, RemoteInbox: inboxPtr, ActivityID: stored.ID})
		return err
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &CreateFollowingResult{Account: *account, RawJSON: raw, Inbox: input.Inbox}, nil
}
