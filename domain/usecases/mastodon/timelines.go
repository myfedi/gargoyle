package mastodon

import (
	"context"
	"sort"
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
	following, err := u.cfg.FollowsRepo.ListFollowing(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	actors := make([]string, 0, len(following)+1)
	actors = append(actors, account.URI)
	for _, follow := range following {
		actors = append(actors, follow.RemoteActor)
	}
	notes, err := u.cfg.NotesRepo.ListHomeTimelineNotesPaged(ctx, nil, account.ID, actors, opts.Limit, opts.MaxID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	items, derr := u.timelineItems(ctx, account, notes)
	if derr != nil {
		return nil, derr
	}
	boosts, err := u.cfg.BoostsRepo.ListTimelineBoosts(ctx, nil, account.ID, opts.Limit, opts.MaxID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	boostItems, derr := u.boostTimelineItems(ctx, account, boosts)
	if derr != nil {
		return nil, derr
	}
	actorSet := make(map[string]bool, len(actors))
	for _, actor := range actors {
		actorSet[actor] = true
	}
	boostItems = filterTimelineItemsByActor(boostItems, actorSet)
	return mergeTimelineItems(items, boostItems, opts.Limit), nil
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
		item, derr := u.timelineItem(ctx, localAccount, note, *author, replyAccountID, media)
		if derr != nil {
			return nil, derr
		}
		items = append(items, *item)
	}
	return items, nil
}

func (u UseCase) timelineItem(ctx context.Context, localAccount *models.Account, note models.Note, author models.Account, replyAccountID *string, media []models.MediaAttachment) (*TimelineItem, *domainerrors.DomainError) {
	reblogsCount, err := u.cfg.BoostsRepo.CountBoostsForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	reblogged, err := u.cfg.BoostsRepo.BoostExists(ctx, nil, localAccount.ID, localAccount.URI, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	favourited, err := u.cfg.SocialRepo.InteractionExists(ctx, nil, localAccount.ID, note.ID, "favourite")
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	bookmarked, err := u.cfg.SocialRepo.InteractionExists(ctx, nil, localAccount.ID, note.ID, "bookmark")
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	pinned := false
	if author.UserID != nil && note.AttributedTo == author.URI {
		pinned, err = u.cfg.SocialRepo.InteractionExists(ctx, nil, author.ID, note.ID, "pin")
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
	}
	mentions, err := u.cfg.MentionsRepo.ListMentionsForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &TimelineItem{ID: note.ID, URI: note.URI, CreatedAt: note.PublishedAt, Note: note, Account: author, InReplyToAccountID: replyAccountID, Media: media, Mentions: mentions, ReblogsCount: reblogsCount, Reblogged: reblogged, Favourited: favourited, Bookmarked: bookmarked, Pinned: pinned}, nil
}

func (u UseCase) boostTimelineItems(ctx context.Context, localAccount *models.Account, boosts []models.Boost) ([]TimelineItem, *domainerrors.DomainError) {
	items := make([]TimelineItem, 0, len(boosts))
	for _, boost := range boosts {
		note, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, boost.NoteID)
		if err != nil {
			continue
		}
		originalAuthor, derr := u.noteAuthor(ctx, localAccount, *note)
		if derr != nil {
			return nil, derr
		}
		media, err := u.cfg.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		inner, derr := u.timelineItem(ctx, localAccount, *note, *originalAuthor, u.replyAccountID(ctx, localAccount, *note), media)
		if derr != nil {
			return nil, derr
		}
		booster, derr := u.accountForActor(ctx, localAccount, boost.Actor)
		if derr != nil {
			return nil, derr
		}
		items = append(items, TimelineItem{ID: boost.ID, URI: boost.URI, CreatedAt: boost.PublishedAt, Note: *note, Account: *booster, Reblog: inner})
	}
	return items, nil
}

func (u UseCase) accountForActor(ctx context.Context, localAccount *models.Account, actor string) (*models.Account, *domainerrors.DomainError) {
	if actor == localAccount.URI {
		return localAccount, nil
	}
	prefix := strings.TrimRight(u.cfg.Host, "/") + "/users/"
	if strings.HasPrefix(actor, prefix) {
		account, err := u.cfg.AccountsRepo.GetLocalAccountByUsername(ctx, nil, strings.TrimPrefix(actor, prefix))
		if err == nil {
			return account, nil
		}
	}
	remote, err := u.cfg.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, actor)
	if err == nil {
		return remote, nil
	}
	remote, err = u.resolveAndCacheRemoteAccount(ctx, actor, localAccount)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrNotFound, err)
	}
	return remote, nil
}

func filterTimelineItemsByActor(items []TimelineItem, actors map[string]bool) []TimelineItem {
	filtered := make([]TimelineItem, 0, len(items))
	for _, item := range items {
		if actors[item.Account.URI] {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func mergeTimelineItems(a []TimelineItem, b []TimelineItem, limit int) []TimelineItem {
	items := append(a, b...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
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
