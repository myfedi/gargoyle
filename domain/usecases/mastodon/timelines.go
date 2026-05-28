package mastodon

import (
	"context"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

// HomeTimeline returns notes addressed to the authenticated account. Each item
// carries the account that authored the note so Mastodon responses can render
// remote statuses as remote authors instead of as the local timeline owner.
func (u UseCase) HomeTimeline(ctx context.Context, account *models.Account, opts TimelineOptions) ([]TimelineItem, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	opts = normalizeTimelineOptions(opts)
	notes, err := u.cfg.NotesRepo.ListLocalNotesPaged(ctx, nil, account.ID, opts.Limit, opts.MaxID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.timelineItems(ctx, account, notes)
}

// PublicTimeline returns the server-known public timeline for the authenticated
// account. With the current read model, "known" means notes stored for that
// local account: local posts plus remote posts delivered through federation.
func (u UseCase) PublicTimeline(ctx context.Context, account *models.Account, opts TimelineOptions) ([]TimelineItem, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	opts = normalizeTimelineOptions(opts)
	prefix := strings.TrimRight(u.cfg.Host, "/") + "/users/"
	var notes []models.Note
	var err error
	switch {
	case opts.LocalOnly:
		notes, err = u.cfg.NotesRepo.ListKnownLocalTimelineNotesPaged(ctx, nil, account.ID, prefix, opts.Limit, opts.MaxID)
	case opts.RemoteOnly:
		notes, err = u.cfg.NotesRepo.ListKnownRemoteTimelineNotesPaged(ctx, nil, account.ID, prefix, opts.Limit, opts.MaxID)
	default:
		notes, err = u.cfg.NotesRepo.ListKnownPublicTimelineNotesPaged(ctx, nil, account.ID, opts.Limit, opts.MaxID)
	}
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.timelineItems(ctx, account, notes)
}

func (u UseCase) timelineItems(ctx context.Context, localAccount *models.Account, notes []models.Note) ([]TimelineItem, *domainerrors.DomainError) {
	items := make([]TimelineItem, 0, len(notes))
	for _, note := range notes {
		author, derr := u.noteAuthor(ctx, localAccount, note)
		if derr != nil {
			return nil, derr
		}
		replyAccountID := u.replyAccountID(ctx, localAccount, note)
		media, err := u.cfg.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		items = append(items, TimelineItem{Note: note, Account: *author, InReplyToAccountID: replyAccountID, Media: media})
	}
	return items, nil
}

func (u UseCase) replyAccountID(ctx context.Context, localAccount *models.Account, note models.Note) *string {
	if note.InReplyToID == nil || *note.InReplyToID == "" {
		return nil
	}
	parent, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, *note.InReplyToID)
	if err != nil {
		return nil
	}
	author, derr := u.noteAuthor(ctx, localAccount, *parent)
	if derr != nil {
		return nil
	}
	return &author.ID
}

func normalizeTimelineOptions(opts TimelineOptions) TimelineOptions {
	if opts.Limit <= 0 || opts.Limit > 40 {
		opts.Limit = 20
	}
	if opts.LocalOnly {
		opts.RemoteOnly = false
	}
	return opts
}

func (u UseCase) noteAuthor(ctx context.Context, localAccount *models.Account, note models.Note) (*models.Account, *domainerrors.DomainError) {
	if note.AttributedTo == "" || note.AttributedTo == localAccount.URI {
		return localAccount, nil
	}
	remote, err := u.cfg.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, note.AttributedTo)
	if err == nil {
		return remote, nil
	}
	remote, err = u.resolveAndCacheRemoteAccount(ctx, note.AttributedTo, localAccount)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return remote, nil
}
