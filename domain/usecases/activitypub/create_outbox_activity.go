package activitypub

import (
	"context"
	"database/sql"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// CreateOutboxActivityInput contains a local actor outbox submission.
type CreateOutboxActivityInput struct {
	Username   string
	RawJSON    []byte
	ActivityID string
	ObjectID   string
}

// CreateOutboxActivityResult contains committed local state plus delivery targets
// for infrastructure fan-out after the transaction has completed.
type CreateOutboxActivityResult struct {
	Account         models.Account
	RawJSON         []byte
	FollowerInboxes []string
}

// CreateOutboxActivity normalizes and persists a local outbox activity, creating
// any derived Note in the same transaction, then returns follower inboxes to notify.
func (u *CreateOutboxActivityUseCase) CreateOutboxActivity(ctx context.Context, input CreateOutboxActivityInput) (*CreateOutboxActivityResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, input.Username)
	if derr != nil {
		return nil, derr
	}
	raw, derr := NormalizeOutboxActivity(input.RawJSON, *account, input.ActivityID, input.ObjectID, u.cfg.ContentSanitizer)
	if derr != nil {
		return nil, derr
	}
	activity, derr := ParseActivity(raw)
	if derr != nil {
		return nil, derr
	}
	if activity.Actor != "" && activity.Actor != account.URI {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "activity actor does not match local actor")
	}

	err := u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		stored, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionOutbox, Type: activity.Type, Actor: account.URI, Object: activity.Object, RawJSON: string(raw)})
		if err != nil {
			return err
		}
		if u.cfg.NotesRepo != nil {
			if note, ok := ExtractNote(raw); ok {
				_, err := u.cfg.NotesRepo.CreateNote(ctx, &tx, repos.CreateNoteInput{LocalAccountID: account.ID, ActivityID: stored.ID, URI: note.URI, Content: u.cfg.ContentSanitizer.SanitizeHTML(note.Content), PlainText: u.cfg.ContentSanitizer.StripHTMLFromText(note.Content), AttributedTo: note.AttributedTo, PublishedAt: note.PublishedAt})
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	var inboxes []string
	if u.cfg.FollowsRepo != nil {
		followers, err := u.cfg.FollowsRepo.ListFollowers(ctx, nil, account.ID)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		for _, follower := range followers {
			if follower.RemoteInbox != nil {
				inboxes = append(inboxes, *follower.RemoteInbox)
			}
		}
	}
	return &CreateOutboxActivityResult{Account: *account, RawJSON: raw, FollowerInboxes: inboxes}, nil
}
