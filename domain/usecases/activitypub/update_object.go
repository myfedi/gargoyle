package activitypub

import (
	"context"
	"database/sql"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// UpdateObjectInput contains AP command data for updating a local object.
type UpdateObjectInput struct {
	Username    string
	ObjectID    string
	UpdatedNote models.Note
	RawJSON     []byte
	Media       []models.MediaAttachment
	Mentions    []models.Account
	PollOptions []string
}

// UpdateObjectResult contains the committed Update payload and delivery data.
type UpdateObjectResult struct {
	Account         models.Account
	Note            models.Note
	Mentions        []models.Mention
	RawJSON         []byte
	FollowerInboxes []string
	MentionInboxes  []string
}

// UpdateObject stores an outbound Update activity and updates local object metadata in one transaction.
func (u *UpdateObjectUseCase) UpdateObject(ctx context.Context, input UpdateObjectInput) (*UpdateObjectResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, input.Username)
	if derr != nil {
		return nil, derr
	}
	if input.ObjectID == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "object id is required")
	}
	if len(input.RawJSON) == 0 {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "activity is required")
	}
	current, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, input.ObjectID)
	if err != nil || current.LocalAccountID != account.ID || current.AttributedTo != account.URI {
		return nil, domainerrors.New(domainerrors.ErrNotFound, objectNotFoundMessage)
	}
	updated := input.UpdatedNote
	updated.ID = current.ID
	updated.URI = current.URI
	updated.LocalAccountID = current.LocalAccountID
	updated.ActivityID = current.ActivityID
	updated.AttributedTo = current.AttributedTo
	updated.PublishedAt = current.PublishedAt
	updated.CreatedAt = current.CreatedAt
	updated.InReplyToID = current.InReplyToID
	updated.InReplyToURI = current.InReplyToURI

	var stored *models.Note
	var storedMentions []models.Mention
	err = u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		if err := u.storeCurrentObjectRevision(ctx, &tx, current.ID); err != nil {
			return err
		}
		note, err := u.cfg.NotesRepo.UpdateNoteByID(ctx, &tx, current.ID, repos.UpdateNoteInput{Content: updated.Content, PlainText: updated.PlainText, ObjectType: updated.ObjectType, Visibility: updated.Visibility, PollMultiple: updated.PollMultiple, PollExpiresAt: updated.PollExpiresAt, Hashtags: updated.Hashtags, Emojis: updated.Emojis, Sensitive: updated.Sensitive, SpoilerText: updated.SpoilerText})
		if err != nil {
			return err
		}
		stored = note
		mediaIDs := make([]string, 0, len(input.Media))
		for _, item := range input.Media {
			mediaIDs = append(mediaIDs, item.ID)
		}
		if err := u.cfg.MediaRepo.ReplaceMediaForNote(ctx, &tx, current.ID, mediaIDs); err != nil {
			return err
		}
		if _, err := u.cfg.PollsRepo.ReplacePoll(ctx, &tx, repos.CreatePollInput{NoteID: current.ID, Options: input.PollOptions, Multiple: updated.PollMultiple}); err != nil {
			return err
		}
		mentions, err := u.replaceObjectMentions(ctx, &tx, *account, current.ID, input.Mentions)
		if err != nil {
			return err
		}
		storedMentions = mentions
		_, err = u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionOutbox, Type: "Update", Actor: account.URI, Object: current.URI, RawJSON: string(input.RawJSON)})
		return err
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	inboxes, derr := updateObjectFollowerInboxes(ctx, u.cfg.FollowsRepo, account.ID)
	if derr != nil {
		return nil, derr
	}
	return &UpdateObjectResult{Account: *account, Note: *stored, Mentions: storedMentions, RawJSON: input.RawJSON, FollowerInboxes: inboxes, MentionInboxes: mentionInboxes(input.Mentions)}, nil
}

func (u *UpdateObjectUseCase) storeCurrentObjectRevision(ctx context.Context, tx *db.Tx, noteID string) error {
	note, err := u.cfg.NotesRepo.GetNoteByID(ctx, tx, noteID)
	if err != nil {
		return err
	}
	media, err := u.cfg.MediaRepo.ListMediaForNote(ctx, tx, noteID)
	if err != nil {
		return err
	}
	mediaIDs := make([]string, 0, len(media))
	for _, item := range media {
		mediaIDs = append(mediaIDs, item.ID)
	}
	createdAt := note.PublishedAt
	if note.EditedAt != nil {
		createdAt = *note.EditedAt
	}
	_, err = u.cfg.NotesRepo.CreateNoteEdit(ctx, tx, repos.CreateNoteEditInput{Note: *note, CreatedAt: createdAt, MediaIDs: mediaIDs})
	return err
}

func updateObjectFollowerInboxes(ctx context.Context, repo repos.FollowsRepository, accountID string) ([]string, *domainerrors.DomainError) {
	followers, err := repo.ListFollowers(ctx, nil, accountID)
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

func (u *UpdateObjectUseCase) replaceObjectMentions(ctx context.Context, tx *db.Tx, account models.Account, noteID string, mentions []models.Account) ([]models.Mention, error) {
	if err := u.cfg.MentionsRepo.DeleteMentionsForNote(ctx, tx, noteID); err != nil {
		return nil, err
	}
	for _, mention := range mentions {
		input := repos.CreateMentionInput{LocalAccountID: account.ID, NoteID: noteID, AccountID: mention.ID, Username: mention.Username, Acct: mentionAcct(mention), URL: mentionURL(mention, u.cfg.Host)}
		if _, err := u.cfg.MentionsRepo.CreateMention(ctx, tx, input); err != nil {
			return nil, err
		}
	}
	return u.cfg.MentionsRepo.ListMentionsForNote(ctx, tx, noteID)
}
