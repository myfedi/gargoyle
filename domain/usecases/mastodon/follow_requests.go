package mastodon

import (
	"context"
	"database/sql"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	activitypubUC "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

type FollowRequestDecisionResult struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

func (u UseCase) FollowRequests(ctx context.Context, localAccount *models.Account) ([]models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	follows, err := u.cfg.FollowsRepo.ListPendingFollowers(ctx, nil, localAccount.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.resolveFollowActors(ctx, localAccount, follows)
}

func (u UseCase) AuthorizeFollowRequest(ctx context.Context, localAccount *models.Account, accountID string) (*FollowRequestDecisionResult, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	actor, err := RemoteActorFromAccountID(accountID)
	if err != nil || actor == "" || actor == localAccount.ID {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "invalid follow request account")
	}

	var follow *models.Follow
	var activity *models.Activity
	err = u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		var err error
		follow, err = u.cfg.FollowsRepo.AcceptFollowByActor(ctx, &tx, localAccount.ID, actor)
		if err != nil {
			return err
		}
		activity, err = u.cfg.ActivitiesRepo.GetActivityByID(ctx, &tx, follow.ActivityID)
		if err != nil {
			return err
		}
		return u.cfg.SocialRepo.DeleteNotificationsByActorAndType(ctx, &tx, localAccount.ID, actor, "follow_request")
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	raw, err := activitypubUC.MarshalAccept(*localAccount, *follow, []byte(activity.RawJSON))
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	inbox := ""
	if follow.RemoteInbox != nil {
		inbox = *follow.RemoteInbox
	}
	return &FollowRequestDecisionResult{Account: *localAccount, RawJSON: raw, Inbox: inbox}, nil
}

func (u UseCase) RejectFollowRequest(ctx context.Context, localAccount *models.Account, accountID string) (*FollowRequestDecisionResult, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	actor, err := RemoteActorFromAccountID(accountID)
	if err != nil || actor == "" || actor == localAccount.ID {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "invalid follow request account")
	}

	var follow *models.Follow
	var activity *models.Activity
	err = u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		var err error
		follow, err = u.cfg.FollowsRepo.GetFollowByActor(ctx, &tx, localAccount.ID, actor, "follower")
		if err != nil {
			return err
		}
		activity, err = u.cfg.ActivitiesRepo.GetActivityByID(ctx, &tx, follow.ActivityID)
		if err != nil {
			return err
		}
		if err := u.cfg.FollowsRepo.DeleteFollowByActor(ctx, &tx, localAccount.ID, actor); err != nil {
			return err
		}
		return u.cfg.SocialRepo.DeleteNotificationsByActorAndType(ctx, &tx, localAccount.ID, actor, "follow_request")
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	raw, err := activitypubUC.MarshalReject(*localAccount, *follow, []byte(activity.RawJSON))
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	inbox := ""
	if follow.RemoteInbox != nil {
		inbox = *follow.RemoteInbox
	}
	return &FollowRequestDecisionResult{Account: *localAccount, RawJSON: raw, Inbox: inbox}, nil
}
