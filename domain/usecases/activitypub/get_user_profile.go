package activitypub

import (
	"context"
	"database/sql"
	"errors"

	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/activitypub"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type GetUserProfileUseCaseConfig struct {
	AccountsRepo repos.AccountsRepo
	Serializer   activitypub.ActorSerializer
}

type GetUserProfileUseCase struct {
	cfg GetUserProfileUseCaseConfig
}

func NewGetUserProfileUseCase(cfg GetUserProfileUseCaseConfig) GetUserProfileUseCase {
	if cfg.AccountsRepo == nil {
		panic("get user profile use case requires AccountsRepo")
	}
	if cfg.Serializer == nil {
		panic("get user profile use case requires Serializer")
	}
	return GetUserProfileUseCase{
		cfg: cfg,
	}
}

func (u *GetUserProfileUseCase) GetUserProfile(ctx context.Context, username string) (string, *domainerrors.DomainError) {
	account, err := u.cfg.AccountsRepo.GetLocalAccountByUsername(ctx, nil, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", domainerrors.New(domainerrors.ErrNotFound, "no such username")
		}
		return "", domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	res, err := u.cfg.Serializer.Marshall(*account)
	if err != nil {
		return "", domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	return res, nil
}
