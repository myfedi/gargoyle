package mastodon

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

// CreateStatus creates a local Note through the ActivityPub outbox workflow so
// Mastodon API posting and federation posting share the same normalization,
// persistence, and fan-out semantics.
func (u UseCase) CreateStatus(ctx context.Context, account *models.Account, content string) (*CreateStatusResult, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	if strings.TrimSpace(content) == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "status is required")
	}
	activityID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	objectID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	raw, err := json.Marshal(map[string]any{"type": "Note", "content": content})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res, derr := u.cfg.CreateOutboxUC.CreateOutboxActivity(ctx, apUsecases.CreateOutboxActivityInput{Username: account.Username, RawJSON: raw, ActivityID: activityID, ObjectID: objectID})
	if derr != nil {
		return nil, derr
	}
	extracted, ok := apUsecases.ExtractNote(res.RawJSON)
	if !ok {
		return nil, domainerrors.New(domainerrors.ErrInternal, "created activity did not contain a note")
	}
	note, err := u.cfg.NotesRepo.GetNoteByURI(ctx, nil, extracted.URI)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &CreateStatusResult{Note: *note, Account: res.Account, RawJSON: res.RawJSON, FollowerInboxes: res.FollowerInboxes}, nil
}
