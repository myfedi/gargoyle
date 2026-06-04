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
	// Normalize before persistence so all local outbox writes use one canonical
	// ActivityPub representation, regardless of which adapter submitted them.
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

	// The use case owns the transaction boundary for local state. Network fan-out
	// is returned to infrastructure after commit through FollowerInboxes.
	err := u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		return u.persistOutboxActivity(ctx, &tx, *account, activity, raw)
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	inboxes, derr := u.followerInboxes(ctx, account.ID)
	if derr != nil {
		return nil, derr
	}
	return &CreateOutboxActivityResult{Account: *account, RawJSON: raw, FollowerInboxes: inboxes}, nil
}

// persistOutboxActivity writes the activity and any derived Note inside the
// caller-owned transaction. It talks only through repository ports, preserving
// the domain/usecase boundary from storage adapters.
func (u *CreateOutboxActivityUseCase) persistOutboxActivity(
	ctx context.Context,
	tx *db.Tx,
	account models.Account,
	activity ParsedActivity,
	raw []byte,
) error {
	stored, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, tx, repos.CreateActivityInput{
		LocalAccountID: account.ID,
		Direction:      models.ActivityDirectionOutbox,
		Type:           activity.Type,
		Actor:          account.URI,
		Object:         activity.Object,
		RawJSON:        string(raw),
	})
	if err != nil {
		return err
	}
	if u.cfg.NotesRepo == nil {
		return nil
	}

	note, ok := ExtractNote(raw)
	if !ok {
		return nil
	}
	// Replies may point at remote objects we do not have yet. Enqueueing a fetch
	// records that dependency without performing network I/O inside the transaction.
	replyID, replyURI := replyIDs(ctx, u.cfg.NotesRepo, tx, note)
	if err := enqueueMissingReplyFetch(ctx, u.cfg.FetchJobsRepo, tx, account.ID, note, replyID); err != nil {
		return err
	}
	created, err := u.cfg.NotesRepo.CreateNote(ctx, tx, repos.CreateNoteInput{
		LocalAccountID: account.ID,
		ActivityID:     stored.ID,
		URI:            note.URI,
		Content:        u.cfg.ContentSanitizer.SanitizeHTML(note.Content),
		PlainText:      u.cfg.ContentSanitizer.StripHTMLFromText(note.Content),
		ObjectType:     note.Type,
		Visibility:     note.Visibility,
		Sensitive:      note.Sensitive,
		SpoilerText:    note.SpoilerText,
		AttributedTo:   note.AttributedTo,
		InReplyToID:    replyID,
		InReplyToURI:   replyURI,
		PublishedAt:    note.PublishedAt,
	})
	if err != nil {
		return err
	}
	return u.createPollOptions(ctx, tx, created.ID, note)
}

// followerInboxes returns delivery targets after commit. Delivery itself stays
// in infrastructure so domain code never performs network side effects.
func (u *CreateOutboxActivityUseCase) followerInboxes(ctx context.Context, accountID string) ([]string, *domainerrors.DomainError) {
	if u.cfg.FollowsRepo == nil {
		return nil, nil
	}
	followers, err := u.cfg.FollowsRepo.ListFollowers(ctx, nil, accountID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	inboxes := make([]string, 0, len(followers))
	for _, follower := range followers {
		if follower.RemoteInbox != nil {
			inboxes = append(inboxes, *follower.RemoteInbox)
		}
	}
	return inboxes, nil
}
