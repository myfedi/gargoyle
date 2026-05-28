package mastodon

import (
	"context"
	"encoding/json"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

type InteractionResult struct {
	Status   TimelineItem
	Delivery *DeliveryPayload
}

type DeliveryPayload struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

func (u UseCase) FavouriteStatus(ctx context.Context, account *models.Account, id string) (*InteractionResult, *domainerrors.DomainError) {
	return u.interact(ctx, account, id, "favourite", "Like")
}
func (u UseCase) UnfavouriteStatus(ctx context.Context, account *models.Account, id string) (*InteractionResult, *domainerrors.DomainError) {
	return u.undoInteract(ctx, account, id, "favourite", "Like")
}
func (u UseCase) ReblogStatus(ctx context.Context, account *models.Account, id string) (*InteractionResult, *domainerrors.DomainError) {
	return u.interact(ctx, account, id, "reblog", "Announce")
}
func (u UseCase) UnreblogStatus(ctx context.Context, account *models.Account, id string) (*InteractionResult, *domainerrors.DomainError) {
	return u.undoInteract(ctx, account, id, "reblog", "Announce")
}

func (u UseCase) interact(ctx context.Context, account *models.Account, id string, typ string, apType string) (*InteractionResult, *domainerrors.DomainError) {
	item, derr := u.GetStatus(ctx, account, id)
	if derr != nil {
		return nil, derr
	}
	if _, err := u.cfg.SocialRepo.CreateInteraction(ctx, nil, account.ID, id, typ); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if item.Account.Domain == nil && item.Account.ID != account.ID {
		if _, err := u.cfg.SocialRepo.CreateNotification(ctx, nil, item.Account.ID, account.URI, typ, &id); err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
	}
	payload, derr := u.interactionPayload(ctx, account, item, apType, false)
	if derr != nil {
		return nil, derr
	}
	return &InteractionResult{Status: *item, Delivery: payload}, nil
}
func (u UseCase) undoInteract(ctx context.Context, account *models.Account, id string, typ string, apType string) (*InteractionResult, *domainerrors.DomainError) {
	item, derr := u.GetStatus(ctx, account, id)
	if derr != nil {
		return nil, derr
	}
	if err := u.cfg.SocialRepo.DeleteInteraction(ctx, nil, account.ID, id, typ); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	payload, derr := u.interactionPayload(ctx, account, item, apType, true)
	if derr != nil {
		return nil, derr
	}
	return &InteractionResult{Status: *item, Delivery: payload}, nil
}
func (u UseCase) interactionPayload(ctx context.Context, account *models.Account, item *TimelineItem, apType string, undo bool) (*DeliveryPayload, *domainerrors.DomainError) {
	if item.Account.ID == account.ID {
		return nil, nil
	}
	activityID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	activity := map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": account.URI + "/activities/" + activityID, "type": apType, "actor": account.URI, "object": item.Note.URI}
	if undo {
		activity = map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": account.URI + "/activities/" + activityID, "type": "Undo", "actor": account.URI, "object": activity}
	}
	raw, err := json.Marshal(activity)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &DeliveryPayload{Account: *account, RawJSON: raw, Inbox: item.Account.InboxURI}, nil
}
