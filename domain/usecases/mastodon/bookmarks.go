package mastodon

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

func (u UseCase) BookmarkStatus(ctx context.Context, account *models.Account, id string) (*TimelineItem, *domainerrors.DomainError) {
	return u.localInteraction(ctx, account, id, "bookmark", true)
}

func (u UseCase) UnbookmarkStatus(ctx context.Context, account *models.Account, id string) (*TimelineItem, *domainerrors.DomainError) {
	return u.localInteraction(ctx, account, id, "bookmark", false)
}

func (u UseCase) FavouriteStatuses(ctx context.Context, account *models.Account, limit int) ([]TimelineItem, *domainerrors.DomainError) {
	return u.interactionStatuses(ctx, account, "favourite", limit)
}

func (u UseCase) BookmarkedStatuses(ctx context.Context, account *models.Account, limit int) ([]TimelineItem, *domainerrors.DomainError) {
	return u.interactionStatuses(ctx, account, "bookmark", limit)
}

func (u UseCase) localInteraction(ctx context.Context, account *models.Account, id, typ string, create bool) (*TimelineItem, *domainerrors.DomainError) {
	item, derr := u.GetStatus(ctx, account, id)
	if derr != nil {
		return nil, derr
	}
	if create {
		if _, err := u.cfg.SocialRepo.CreateInteraction(ctx, nil, account.ID, id, typ); err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
	} else if err := u.cfg.SocialRepo.DeleteInteraction(ctx, nil, account.ID, id, typ); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return item, nil
}

func (u UseCase) interactionStatuses(ctx context.Context, account *models.Account, typ string, limit int) ([]TimelineItem, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	interactions, err := u.cfg.SocialRepo.ListInteractions(ctx, nil, account.ID, typ, limit)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	items := make([]TimelineItem, 0, len(interactions))
	for _, interaction := range interactions {
		item, derr := u.GetStatus(ctx, account, interaction.NoteID)
		if derr == nil {
			items = append(items, *item)
		}
	}
	return items, nil
}
