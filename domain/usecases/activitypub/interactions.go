package activitypub

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// CreateInteractionInput contains AP command data for a local Like or Announce.
type CreateInteractionInput struct {
	Username             string
	ObjectID             string
	ObjectURI            string
	Type                 string
	ActivityID           string
	TargetInbox          string
	TargetLocalAccountID string
}

// CreateInteractionResult contains the committed activity payload and optional inbox
// for infrastructure delivery after the transaction has completed.
type CreateInteractionResult struct {
	Account     models.Account
	RawJSON     []byte
	Inbox       string
	ActivityURI string
}

// UndoInteractionInput contains AP command data for undoing a local Like or Announce.
type UndoInteractionInput struct {
	Username    string
	ObjectID    string
	ObjectURI   string
	Type        string
	ActivityID  string
	UndoID      string
	TargetInbox string
}

// UndoInteractionResult contains the committed Undo payload and optional inbox
// for infrastructure delivery after the transaction has completed.
type UndoInteractionResult struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

func (u *CreateInteractionUseCase) CreateInteraction(ctx context.Context, input CreateInteractionInput) (*CreateInteractionResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, input.Username)
	if derr != nil {
		return nil, derr
	}
	interactionType, derr := interactionStorageType(input.Type)
	if derr != nil {
		return nil, derr
	}
	if input.ObjectID == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "object id is required")
	}
	if input.ObjectURI == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "object URI is required")
	}
	if input.ActivityID == "" {
		return nil, domainerrors.New(domainerrors.ErrInternal, "activity id is required")
	}

	activityURI := account.URI + activityPathSegment + input.ActivityID
	activity := map[string]any{activityStreamsContextKey: activityStreamsContextURI, "id": activityURI, "type": input.Type, "actor": account.URI, "object": input.ObjectURI}
	raw, err := json.Marshal(activity)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	err = u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		if _, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionOutbox, Type: input.Type, Actor: account.URI, Object: input.ObjectURI, RawJSON: string(raw)}); err != nil {
			return err
		}
		if _, err := u.cfg.SocialRepo.CreateInteraction(ctx, &tx, account.ID, input.ObjectID, interactionType); err != nil {
			return err
		}
		if input.Type == "Announce" && u.cfg.BoostsRepo != nil {
			if _, err := u.cfg.BoostsRepo.CreateBoost(ctx, &tx, repos.CreateBoostInput{LocalAccountID: account.ID, Actor: account.URI, NoteID: input.ObjectID, URI: activityURI, PublishedAt: time.Now().UTC()}); err != nil {
				return err
			}
		}
		if input.TargetLocalAccountID != "" && input.TargetLocalAccountID != account.ID {
			if _, err := u.cfg.SocialRepo.CreateNotification(ctx, &tx, input.TargetLocalAccountID, account.URI, interactionType, &input.ObjectID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &CreateInteractionResult{Account: *account, RawJSON: raw, Inbox: input.TargetInbox, ActivityURI: activityURI}, nil
}

func (u *UndoInteractionUseCase) UndoInteraction(ctx context.Context, input UndoInteractionInput) (*UndoInteractionResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, input.Username)
	if derr != nil {
		return nil, derr
	}
	interactionType, derr := interactionStorageType(input.Type)
	if derr != nil {
		return nil, derr
	}
	if input.ObjectID == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "object id is required")
	}
	if input.ObjectURI == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "object URI is required")
	}
	if input.ActivityID == "" || input.UndoID == "" {
		return nil, domainerrors.New(domainerrors.ErrInternal, "activity and undo ids are required")
	}

	activityURI := account.URI + activityPathSegment + input.ActivityID
	activity := map[string]any{activityStreamsContextKey: activityStreamsContextURI, "id": activityURI, "type": input.Type, "actor": account.URI, "object": input.ObjectURI}
	undoActivity := map[string]any{activityStreamsContextKey: activityStreamsContextURI, "id": account.URI + activityPathSegment + input.UndoID, "type": "Undo", "actor": account.URI, "object": activity}
	raw, err := json.Marshal(undoActivity)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	err = u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		if _, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionOutbox, Type: "Undo", Actor: account.URI, Object: input.ObjectURI, RawJSON: string(raw)}); err != nil {
			return err
		}
		if err := u.cfg.SocialRepo.DeleteInteraction(ctx, &tx, account.ID, input.ObjectID, interactionType); err != nil {
			return err
		}
		if input.Type == "Announce" && u.cfg.BoostsRepo != nil {
			if err := u.cfg.BoostsRepo.DeleteBoost(ctx, &tx, account.ID, account.URI, input.ObjectID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &UndoInteractionResult{Account: *account, RawJSON: raw, Inbox: input.TargetInbox}, nil
}

func interactionStorageType(activityType string) (string, *domainerrors.DomainError) {
	switch activityType {
	case "Like":
		return "favourite", nil
	case "Announce":
		return "reblog", nil
	default:
		return "", domainerrors.New(domainerrors.ErrBadRequest, "unsupported interaction type")
	}
}
