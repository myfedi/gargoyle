package clientapi

import (
	"context"
	"time"

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
		if fresh := u.refreshRemoteAccountIfStale(ctx, localAccount, remote); fresh != nil {
			return fresh, nil
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
	isRemoteAccount := account.ID != localAccount.ID
	if isRemoteAccount && maxID == "" {
		u.cacheRemoteOutboxFirstPageAsync(localAccount, *account)
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
		if isRemoteAccount && len(items) < limit {
			if maxID == "" {
				u.cacheRemoteOutboxUntilAsync(localAccount, *account, limit, maxID)
			} else {
				u.cacheRemoteOutboxUntilRequest(ctx, localAccount, *account, func() (bool, error) {
					notes, err := u.deps.NotesRepo.ListAttributedNotesPaged(ctx, nil, localAccount.ID, account.URI, limit, maxID)
					return err == nil && len(notes) >= limit, err
				})
				notes, err = u.accountStatusNotes(ctx, localAccount, account, limit, maxID)
				if err != nil {
					return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
				}
				items, derr = u.accountStatusItems(ctx, localAccount, account, notes)
				if derr != nil {
					return nil, derr
				}
			}
		}
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
	merged := mergeTimelineItems(items, boostItems, limit)
	if isRemoteAccount && len(merged) < limit {
		if maxID == "" {
			u.cacheRemoteOutboxUntilAsync(localAccount, *account, limit, maxID)
		} else {
			u.cacheRemoteOutboxUntilRequest(ctx, localAccount, *account, func() (bool, error) {
				notes, err := u.deps.NotesRepo.ListAttributedNotesPaged(ctx, nil, localAccount.ID, account.URI, limit, maxID)
				if err != nil {
					return false, err
				}
				boosts, err := u.deps.BoostsRepo.ListActorBoosts(ctx, nil, localAccount.ID, account.URI, limit, maxID)
				return err == nil && len(notes)+len(boosts) >= limit, err
			})
			notes, err = u.accountStatusNotes(ctx, localAccount, account, limit, maxID)
			if err != nil {
				return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
			}
			items, derr = u.accountStatusItems(ctx, localAccount, account, notes)
			if derr != nil {
				return nil, derr
			}
			boosts, err = u.deps.BoostsRepo.ListActorBoosts(ctx, nil, localAccount.ID, account.URI, limit, maxID)
			if err != nil {
				return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
			}
			boostItems, derr = u.boostTimelineItems(ctx, localAccount, boosts)
			if derr != nil {
				return nil, derr
			}
			merged = mergeTimelineItems(items, boostItems, limit)
		}
	}
	return merged, nil
}

func (u Accounts) refreshRemoteAccountIfStale(ctx context.Context, signer *models.Account, cached *models.Account) *models.Account {
	if cached == nil || cached.URI == "" || time.Since(cached.FetchedAt) < remoteAccountRefreshAfter {
		return cached
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	fresh, err := u.resolveAndCacheRemoteAccount(refreshCtx, cached.URI, signer)
	if err != nil || fresh == nil {
		return cached
	}
	return fresh
}

func (u Accounts) accountStatusNotes(ctx context.Context, localAccount, account *models.Account, limit int, maxID string) ([]models.Note, error) {
	actor := account.URI
	if account.ID == localAccount.ID {
		actor = localAccount.URI
	}
	return u.deps.NotesRepo.ListAttributedNotesPaged(ctx, nil, localAccount.ID, actor, limit, maxID)
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
