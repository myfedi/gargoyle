package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
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

func (u Interactions) FavouriteStatus(ctx context.Context, account *models.Account, id string) (*InteractionResult, *domainerrors.DomainError) {
	return u.interact(ctx, account, id, "Like")
}
func (u Interactions) UnfavouriteStatus(ctx context.Context, account *models.Account, id string) (*InteractionResult, *domainerrors.DomainError) {
	return u.undoInteract(ctx, account, id, "Like")
}
func (u Interactions) ReblogStatus(ctx context.Context, account *models.Account, id string) (*InteractionResult, *domainerrors.DomainError) {
	return u.interact(ctx, account, id, "Announce")
}
func (u Interactions) UnreblogStatus(ctx context.Context, account *models.Account, id string) (*InteractionResult, *domainerrors.DomainError) {
	return u.undoInteract(ctx, account, id, "Announce")
}

func (u Interactions) interact(ctx context.Context, account *models.Account, id, apType string) (*InteractionResult, *domainerrors.DomainError) {
	item, derr := u.getStatus(ctx, account, id)
	if derr != nil {
		return nil, derr
	}
	activityID, err := u.deps.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	targetLocalAccountID := ""
	if item.Account.Domain == nil {
		targetLocalAccountID = item.Account.ID
	}
	res, derr := u.deps.CreateInteractionUC.CreateInteraction(ctx, apUsecases.CreateInteractionInput{Username: account.Username, ObjectID: id, ObjectURI: item.Note.URI, Type: apType, ActivityID: activityID, TargetInbox: item.Account.InboxURI, TargetLocalAccountID: targetLocalAccountID})
	if derr != nil {
		return nil, derr
	}
	if apType == "Announce" {
		item.Reblogged = true
		item.ReblogsCount++
	}
	return &InteractionResult{Status: *item, Delivery: interactionDelivery(res.Account, res.RawJSON, res.Inbox, item, account)}, nil
}
func (u Interactions) undoInteract(ctx context.Context, account *models.Account, id, apType string) (*InteractionResult, *domainerrors.DomainError) {
	item, derr := u.getStatus(ctx, account, id)
	if derr != nil {
		return nil, derr
	}
	activityID, err := u.deps.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	undoID, err := u.deps.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res, derr := u.deps.UndoInteractionUC.UndoInteraction(ctx, apUsecases.UndoInteractionInput{Username: account.Username, ObjectID: id, ObjectURI: item.Note.URI, Type: apType, ActivityID: activityID, UndoID: undoID, TargetInbox: item.Account.InboxURI})
	if derr != nil {
		return nil, derr
	}
	if apType == "Announce" {
		item.Reblogged = false
		if item.ReblogsCount > 0 {
			item.ReblogsCount--
		}
	}
	return &InteractionResult{Status: *item, Delivery: interactionDelivery(res.Account, res.RawJSON, res.Inbox, item, account)}, nil
}

func interactionDelivery(account models.Account, raw []byte, inbox string, item *TimelineItem, local *models.Account) *DeliveryPayload {
	if item.Account.ID == local.ID {
		return nil
	}
	return &DeliveryPayload{Account: account, RawJSON: raw, Inbox: inbox}
}
