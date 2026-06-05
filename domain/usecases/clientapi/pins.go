package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

func (u Interactions) PinStatus(ctx context.Context, account *models.Account, statusID string) (*TimelineItem, *domainerrors.DomainError) {
	return u.setStatusPin(ctx, account, statusID, true)
}

func (u Interactions) UnpinStatus(ctx context.Context, account *models.Account, statusID string) (*TimelineItem, *domainerrors.DomainError) {
	return u.setStatusPin(ctx, account, statusID, false)
}

func (u Interactions) setStatusPin(ctx context.Context, account *models.Account, statusID string, pinned bool) (*TimelineItem, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	note, err := u.deps.NotesRepo.GetNoteByID(ctx, nil, statusID)
	if err != nil || note.LocalAccountID != account.ID || note.AttributedTo != account.URI {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "status not found")
	}
	if note.Visibility == "direct" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "direct statuses cannot be pinned")
	}
	if pinned {
		if _, err := u.deps.SocialRepo.CreateInteraction(ctx, nil, account.ID, note.ID, "pin"); err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
	} else if err := u.deps.SocialRepo.DeleteInteraction(ctx, nil, account.ID, note.ID, "pin"); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	media, err := u.deps.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.timelineItem(ctx, account, *note, *account, u.replyAccountID(ctx, account, *note), media)
}

func (u Interactions) PinnedAccountStatuses(ctx context.Context, localAccount *models.Account, accountID string, limit int) ([]TimelineItem, *domainerrors.DomainError) {
	account, derr := u.getAccount(ctx, localAccount, accountID)
	if derr != nil {
		return nil, derr
	}
	if account.ID != localAccount.ID {
		return []TimelineItem{}, nil
	}
	interactions, err := u.deps.SocialRepo.ListInteractions(ctx, nil, account.ID, "pin", limit)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	items := make([]TimelineItem, 0, len(interactions))
	for _, interaction := range interactions {
		note, err := u.deps.NotesRepo.GetNoteByID(ctx, nil, interaction.NoteID)
		if err != nil || note.AttributedTo != account.URI {
			continue
		}
		media, err := u.deps.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		item, derr := u.timelineItem(ctx, localAccount, *note, *account, u.replyAccountID(ctx, localAccount, *note), media)
		if derr != nil {
			return nil, derr
		}
		items = append(items, *item)
	}
	return items, nil
}
