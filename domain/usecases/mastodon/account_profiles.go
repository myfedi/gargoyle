package mastodon

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

func (u UseCase) GetAccount(ctx context.Context, localAccount *models.Account, accountID string) (*models.Account, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	if accountID == localAccount.ID {
		return localAccount, nil
	}
	actor, err := RemoteActorFromAccountID(accountID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	remote, err := u.cfg.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, actor)
	if err == nil {
		return remote, nil
	}
	remote, err = u.resolveAndCacheRemoteAccount(ctx, actor, localAccount)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrNotFound, err)
	}
	return remote, nil
}

func (u UseCase) AccountStatuses(ctx context.Context, localAccount *models.Account, accountID string, limit int, maxID string, excludeReblogs bool) ([]TimelineItem, *domainerrors.DomainError) {
	account, derr := u.GetAccount(ctx, localAccount, accountID)
	if derr != nil {
		return nil, derr
	}
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	var notes []models.Note
	var err error
	if account.ID == localAccount.ID {
		notes, err = u.cfg.NotesRepo.ListAttributedNotesPaged(ctx, nil, localAccount.ID, localAccount.URI, limit, maxID)
	} else {
		if maxID == "" {
			_ = u.cacheRemoteOutbox(ctx, localAccount, *account)
		}
		notes, err = u.cfg.NotesRepo.ListAttributedNotesPaged(ctx, nil, localAccount.ID, account.URI, limit, maxID)
	}
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	items := make([]TimelineItem, 0, len(notes))
	for _, note := range notes {
		media, err := u.cfg.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		item, derr := u.timelineItem(ctx, localAccount, note, *account, u.replyAccountID(ctx, localAccount, note), media)
		if derr != nil {
			return nil, derr
		}
		items = append(items, *item)
	}
	if excludeReblogs {
		return items, nil
	}
	boosts, err := u.cfg.BoostsRepo.ListActorBoosts(ctx, nil, localAccount.ID, account.URI, limit, maxID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	boostItems, derr := u.boostTimelineItems(ctx, localAccount, boosts)
	if derr != nil {
		return nil, derr
	}
	return mergeTimelineItems(items, boostItems, limit), nil
}
