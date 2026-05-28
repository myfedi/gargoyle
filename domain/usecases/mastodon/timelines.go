package mastodon

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

// HomeTimeline currently returns locally stored notes for the authenticated
// account. The method boundary is intentionally separate from HTTP so it can be
// expanded into a real followed-account timeline without handler changes.
func (u UseCase) HomeTimeline(ctx context.Context, account *models.Account) ([]models.Note, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	notes, err := u.cfg.NotesRepo.ListLocalNotes(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return notes, nil
}

func (u UseCase) PublicTimeline(ctx context.Context, account *models.Account) ([]models.Note, *domainerrors.DomainError) {
	return u.HomeTimeline(ctx, account)
}
