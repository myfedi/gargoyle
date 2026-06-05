package activitypub

import (
	"context"
	"database/sql"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

// FollowRequestDecisionInput contains AP command data for deciding an inbound Follow.
type FollowRequestDecisionInput struct {
	Username string
	Actor    string
}

// FollowRequestDecisionResult contains the committed Accept/Reject payload and delivery data.
type FollowRequestDecisionResult struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

func (u *FollowRequestDecisionUseCase) Accept(ctx context.Context, input FollowRequestDecisionInput) (*FollowRequestDecisionResult, *domainerrors.DomainError) {
	return u.decide(ctx, input, true)
}

func (u *FollowRequestDecisionUseCase) Reject(ctx context.Context, input FollowRequestDecisionInput) (*FollowRequestDecisionResult, *domainerrors.DomainError) {
	return u.decide(ctx, input, false)
}

func (u *FollowRequestDecisionUseCase) decide(ctx context.Context, input FollowRequestDecisionInput, accept bool) (*FollowRequestDecisionResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, input.Username)
	if derr != nil {
		return nil, derr
	}
	if input.Actor == "" || input.Actor == account.ID || input.Actor == account.URI {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "invalid follow request actor")
	}

	var follow *models.Follow
	var activity *models.Activity
	err := u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		var err error
		if accept {
			follow, err = u.cfg.FollowsRepo.AcceptFollowByActor(ctx, &tx, account.ID, input.Actor)
		} else {
			follow, err = u.cfg.FollowsRepo.GetFollowByActor(ctx, &tx, account.ID, input.Actor, "follower")
		}
		if err != nil {
			return err
		}
		activity, err = u.cfg.ActivitiesRepo.GetActivityByID(ctx, &tx, follow.ActivityID)
		if err != nil {
			return err
		}
		if !accept {
			if err := u.cfg.FollowsRepo.DeleteFollowByActor(ctx, &tx, account.ID, input.Actor); err != nil {
				return err
			}
		}
		return u.cfg.SocialRepo.DeleteNotificationsByActorAndType(ctx, &tx, account.ID, input.Actor, "follow_request")
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	var raw []byte
	if accept {
		raw, err = MarshalAccept(*account, *follow, []byte(activity.RawJSON))
	} else {
		raw, err = MarshalReject(*account, *follow, []byte(activity.RawJSON))
	}
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	inbox := ""
	if follow.RemoteInbox != nil {
		inbox = *follow.RemoteInbox
	}
	return &FollowRequestDecisionResult{Account: *account, RawJSON: raw, Inbox: inbox}, nil
}
