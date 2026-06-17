package activitypub

import (
	"context"
	"database/sql"
	"strings"

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
	Media      []models.MediaAttachment
	Mentions   []models.Account
}

// CreateOutboxActivityResult contains committed local state plus delivery targets
// for infrastructure fan-out after the transaction has completed.
type CreateOutboxActivityResult struct {
	Account         models.Account
	RawJSON         []byte
	Note            *models.Note
	Mentions        []models.Mention
	FollowerInboxes []string
	MentionInboxes  []string
	RelayInboxes    []string
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
	var createdNote *models.Note
	var storedMentions []models.Mention
	err := u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		note, mentions, err := u.persistOutboxActivity(ctx, &tx, *account, activity, raw, input.Media, input.Mentions)
		createdNote = note
		storedMentions = mentions
		return err
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	inboxes, derr := u.followerInboxes(ctx, account.ID)
	if derr != nil {
		return nil, derr
	}
	relayInboxes, derr := u.relayInboxes(ctx, createdNote)
	if derr != nil {
		return nil, derr
	}
	return &CreateOutboxActivityResult{Account: *account, RawJSON: raw, Note: createdNote, Mentions: storedMentions, FollowerInboxes: inboxes, MentionInboxes: mentionInboxes(input.Mentions), RelayInboxes: relayInboxes}, nil
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
	media []models.MediaAttachment,
	mentions []models.Account,
) (*models.Note, []models.Mention, error) {
	stored, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, tx, repos.CreateActivityInput{
		LocalAccountID: account.ID,
		Direction:      models.ActivityDirectionOutbox,
		Type:           activity.Type,
		Actor:          account.URI,
		Object:         activity.Object,
		RawJSON:        string(raw),
	})
	if err != nil {
		return nil, nil, err
	}
	if u.cfg.NotesRepo == nil {
		return nil, nil, nil
	}

	note, ok := ExtractNote(raw)
	if !ok {
		return nil, nil, nil
	}
	// Replies may point at remote objects we do not have yet. Enqueueing a fetch
	// records that dependency without performing network I/O inside the transaction.
	replyID, replyURI := replyIDs(ctx, u.cfg.NotesRepo, tx, note)
	if err := enqueueMissingReplyFetch(ctx, u.cfg.FetchJobsRepo, tx, account.ID, note, replyID); err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}
	if err := u.createPollOptions(ctx, tx, created.ID, note); err != nil {
		return nil, nil, err
	}
	storedMentions, err := u.attachObjectMetadata(ctx, tx, account, created.ID, media, mentions)
	if err != nil {
		return nil, nil, err
	}
	return created, storedMentions, nil
}

// followerInboxes returns delivery targets after commit. Delivery itself stays
// in infrastructure so domain code never performs network side effects.
func (u *CreateOutboxActivityUseCase) attachObjectMetadata(ctx context.Context, tx *db.Tx, account models.Account, noteID string, media []models.MediaAttachment, mentions []models.Account) ([]models.Mention, error) {
	if u.cfg.MediaRepo != nil {
		for _, item := range media {
			if err := u.cfg.MediaRepo.AttachMediaToNote(ctx, tx, noteID, item.ID); err != nil {
				return nil, err
			}
		}
	}
	if u.cfg.MentionsRepo != nil {
		for _, mention := range mentions {
			input := repos.CreateMentionInput{LocalAccountID: account.ID, NoteID: noteID, AccountID: mention.ID, Username: mention.Username, Acct: mentionAcct(mention), URL: mentionURL(mention, u.cfg.Host)}
			if _, err := u.cfg.MentionsRepo.CreateMention(ctx, tx, input); err != nil {
				return nil, err
			}
		}
	}
	if u.cfg.SocialRepo != nil {
		for _, mention := range mentions {
			if mention.Domain != nil || mention.ID == account.ID {
				continue
			}
			if _, err := u.cfg.SocialRepo.CreateNotification(ctx, tx, mention.ID, account.URI, "mention", &noteID); err != nil {
				return nil, err
			}
		}
	}
	if u.cfg.MentionsRepo == nil {
		return nil, nil
	}
	return u.cfg.MentionsRepo.ListMentionsForNote(ctx, tx, noteID)
}

func mentionAcct(account models.Account) string {
	if account.Domain != nil && *account.Domain != "" {
		return account.Username + "@" + *account.Domain
	}
	return account.Username
}

func mentionURL(account models.Account, host string) string {
	if account.URL != nil && *account.URL != "" {
		return *account.URL
	}
	if account.Domain == nil {
		return strings.TrimRight(host, "/") + "/@" + account.Username
	}
	return account.URI
}

func mentionInboxes(mentions []models.Account) []string {
	inboxes := make([]string, 0, len(mentions))
	seen := map[string]bool{}
	for _, mention := range mentions {
		if mention.Domain == nil || mention.InboxURI == "" || seen[mention.InboxURI] {
			continue
		}
		seen[mention.InboxURI] = true
		inboxes = append(inboxes, mention.InboxURI)
	}
	return inboxes
}

func (u *CreateOutboxActivityUseCase) relayInboxes(ctx context.Context, note *models.Note) ([]string, *domainerrors.DomainError) {
	if !u.cfg.RelaysEnabled || u.cfg.RelaysRepo == nil || note == nil || note.Visibility != "public" {
		return nil, nil
	}
	relays, err := u.cfg.RelaysRepo.ListAcceptedRelaySubscriptions(ctx, nil)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	inboxes := make([]string, 0, len(relays))
	for _, relay := range relays {
		if relay.InboxURI != "" {
			inboxes = append(inboxes, relay.InboxURI)
		}
	}
	return inboxes, nil
}

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
