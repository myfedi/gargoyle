package activitypub

import (
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
	return GetUserProfileUseCase{
		cfg: cfg,
	}
}

func (u *GetUserProfileUseCase) GetUserProfile(username string) (string, *domainerrors.DomainError) {
	account, err := u.cfg.AccountsRepo.GetLocalAccountByUsername(nil, username)
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
