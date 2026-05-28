package activitypub

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

// OutboxResult is the application-level representation of a local actor outbox.
type OutboxResult struct {
	Account    models.Account
	Activities []models.Activity
	Total      int
}

// FollowersResult is the application-level representation of a followers collection.
type FollowersResult struct {
	Account   models.Account
	Followers []models.Follow
	Total     int
}

// FollowingResult is the application-level representation of a following collection.
type FollowingResult struct {
	Account   models.Account
	Following []models.Follow
}

// GetOutbox loads a local actor's outbox page and total count.
func (u *GetOutboxUseCase) GetOutbox(ctx context.Context, username string, page PaginationInput) (*OutboxResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, username)
	if derr != nil {
		return nil, derr
	}
	activities, err := u.cfg.ActivitiesRepo.ListPublicOutboxActivitiesPaged(ctx, nil, account.ID, page.Limit, page.Offset)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	total, err := u.cfg.ActivitiesRepo.CountPublicOutboxActivities(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &OutboxResult{Account: *account, Activities: activities, Total: total}, nil
}

// GetFollowers loads a local actor's followers page and total count.
func (u *GetFollowersUseCase) GetFollowers(ctx context.Context, username string, page PaginationInput) (*FollowersResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, username)
	if derr != nil {
		return nil, derr
	}
	followers, err := u.cfg.FollowsRepo.ListFollowersPaged(ctx, nil, account.ID, page.Limit, page.Offset)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	total, err := u.cfg.FollowsRepo.CountFollowers(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &FollowersResult{Account: *account, Followers: followers, Total: total}, nil
}

// GetFollowing loads the accepted remote actors followed by a local actor.
func (u *GetFollowingUseCase) GetFollowing(ctx context.Context, username string) (*FollowingResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, username)
	if derr != nil {
		return nil, derr
	}
	following, err := u.cfg.FollowsRepo.ListFollowing(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &FollowingResult{Account: *account, Following: following}, nil
}
