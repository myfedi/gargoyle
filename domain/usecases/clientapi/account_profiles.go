package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

func (u Accounts) GetAccount(ctx context.Context, localAccount *models.Account, accountID string) (*models.Account, *domainerrors.DomainError) {
	return u.getAccount(ctx, localAccount, accountID)
}

func (u Accounts) getAccount(ctx context.Context, localAccount *models.Account, accountID string) (*models.Account, *domainerrors.DomainError) {
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
	remote, err := u.deps.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, actor)
	if err == nil {
		if derr := u.ensureRemoteDomainAllowed(ctx, remote.URI); derr != nil {
			return nil, derr
		}
		return remote, nil
	}
	remote, err = u.resolveAndCacheRemoteAccount(ctx, actor, localAccount)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrNotFound, err)
	}
	if derr := u.ensureRemoteDomainAllowed(ctx, remote.URI); derr != nil {
		return nil, derr
	}
	return remote, nil
}

func (u Accounts) AccountStatuses(ctx context.Context, localAccount *models.Account, accountID string, limit int, maxID string, excludeReblogs bool) ([]TimelineItem, *domainerrors.DomainError) {
	account, derr := u.getAccount(ctx, localAccount, accountID)
	if derr != nil {
		return nil, derr
	}
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	notes, err := u.accountStatusNotes(ctx, localAccount, account, limit, maxID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	items, derr := u.accountStatusItems(ctx, localAccount, account, notes)
	if derr != nil {
		return nil, derr
	}
	if excludeReblogs {
		return items, nil
	}
	boosts, err := u.deps.BoostsRepo.ListActorBoosts(ctx, nil, localAccount.ID, account.URI, limit, maxID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	boostItems, derr := u.boostTimelineItems(ctx, localAccount, boosts)
	if derr != nil {
		return nil, derr
	}
	return mergeTimelineItems(items, boostItems, limit), nil
}

func (u Accounts) accountStatusNotes(ctx context.Context, localAccount, account *models.Account, limit int, maxID string) ([]models.Note, error) {
	if account.ID == localAccount.ID {
		return u.deps.NotesRepo.ListAttributedNotesPaged(ctx, nil, localAccount.ID, localAccount.URI, limit, maxID)
	}
	notes, err := u.deps.NotesRepo.ListAttributedNotesPaged(ctx, nil, localAccount.ID, account.URI, limit, maxID)
	if err != nil {
		return notes, err
	}
	enough := func() (bool, error) {
		notes, err = u.deps.NotesRepo.ListAttributedNotesPaged(ctx, nil, localAccount.ID, account.URI, limit, maxID)
		if err != nil {
			return false, err
		}
		boosts, err := u.deps.BoostsRepo.ListActorBoosts(ctx, nil, localAccount.ID, account.URI, limit, maxID)
		return err == nil && len(notes)+len(boosts) >= limit, err
	}
	ok, err := enough()
	if err != nil || ok {
		return notes, err
	}
	derr := u.cacheRemoteOutboxUntil(ctx, localAccount, *account, enough)
	if derr != nil {
		return notes, nil
	}
	return u.deps.NotesRepo.ListAttributedNotesPaged(ctx, nil, localAccount.ID, account.URI, limit, maxID)
}

func (u Accounts) accountStatusItems(ctx context.Context, localAccount, account *models.Account, notes []models.Note) ([]TimelineItem, *domainerrors.DomainError) {
	items := make([]TimelineItem, 0, len(notes))
	for _, note := range notes {
		media, err := u.deps.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		item, derr := u.timelineItem(ctx, localAccount, note, *account, u.replyAccountID(ctx, localAccount, note), media)
		if derr != nil {
			return nil, derr
		}
		items = append(items, *item)
	}
	return items, nil
}
