package activitypub

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// DeleteObjectInput contains AP command data for deleting a local object.
type DeleteObjectInput struct {
	Username string
	ObjectID string
	DeleteID string
}

// DeleteObjectResult contains the committed Delete payload and delivery data.
type DeleteObjectResult struct {
	Account         models.Account
	RawJSON         []byte
	FollowerInboxes []string
	Media           []models.MediaAttachment
}

// DeleteObject stores an outbound Delete activity and removes the local object in one transaction.
// Media files are returned for cleanup after commit by an adapter/client layer.
func (u *DeleteObjectUseCase) DeleteObject(ctx context.Context, input DeleteObjectInput) (*DeleteObjectResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, input.Username)
	if derr != nil {
		return nil, derr
	}
	if input.ObjectID == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "object id is required")
	}
	if input.DeleteID == "" {
		return nil, domainerrors.New(domainerrors.ErrInternal, "delete id is required")
	}
	note, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, input.ObjectID)
	if err != nil || note.LocalAccountID != account.ID || note.AttributedTo != account.URI {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "object not found")
	}
	media, err := u.cfg.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	followers, err := u.cfg.FollowsRepo.ListFollowers(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	deleteActivity := map[string]any{activityStreamsContextKey: activityStreamsContextURI, "id": account.URI + "/deletes/" + input.DeleteID, "type": "Delete", "actor": account.URI, "object": note.URI}
	raw, err := json.Marshal(deleteActivity)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	err = u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		if _, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionOutbox, Type: "Delete", Actor: account.URI, Object: note.URI, RawJSON: string(raw)}); err != nil {
			return err
		}
		return u.cfg.NotesRepo.DeleteNoteByID(ctx, &tx, note.ID)
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	inboxes := make([]string, 0, len(followers))
	for _, follower := range followers {
		if follower.RemoteInbox != nil {
			inboxes = append(inboxes, *follower.RemoteInbox)
		}
	}
	return &DeleteObjectResult{Account: *account, RawJSON: raw, FollowerInboxes: inboxes, Media: media}, nil
}
