package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	activitypubUC "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

type FollowRequestDecisionResult struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

func (u Accounts) FollowRequests(ctx context.Context, localAccount *models.Account) ([]models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	follows, err := u.deps.FollowsRepo.ListPendingFollowers(ctx, nil, localAccount.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.resolveFollowActors(ctx, localAccount, follows)
}

func (u Accounts) AuthorizeFollowRequest(ctx context.Context, localAccount *models.Account, accountID string) (*FollowRequestDecisionResult, *domainerrors.DomainError) {
	actor, derr := followRequestActor(localAccount, accountID)
	if derr != nil {
		return nil, derr
	}
	res, derr := u.deps.FollowDecisionUC.Accept(ctx, activitypubUC.FollowRequestDecisionInput{Username: localAccount.Username, Actor: actor})
	if derr != nil {
		return nil, derr
	}
	return &FollowRequestDecisionResult{Account: res.Account, RawJSON: res.RawJSON, Inbox: res.Inbox}, nil
}

func (u Accounts) RejectFollowRequest(ctx context.Context, localAccount *models.Account, accountID string) (*FollowRequestDecisionResult, *domainerrors.DomainError) {
	actor, derr := followRequestActor(localAccount, accountID)
	if derr != nil {
		return nil, derr
	}
	res, derr := u.deps.FollowDecisionUC.Reject(ctx, activitypubUC.FollowRequestDecisionInput{Username: localAccount.Username, Actor: actor})
	if derr != nil {
		return nil, derr
	}
	return &FollowRequestDecisionResult{Account: res.Account, RawJSON: res.RawJSON, Inbox: res.Inbox}, nil
}

func followRequestActor(localAccount *models.Account, accountID string) (string, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return "", derr
	}
	actor, err := RemoteActorFromAccountID(accountID)
	if err != nil || actor == "" || actor == localAccount.ID {
		return "", domainerrors.New(domainerrors.ErrBadRequest, "invalid follow request account")
	}
	return actor, nil
}
