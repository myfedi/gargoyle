package clientapi

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

// HomeTimeline returns notes addressed to the authenticated account. Each item
// carries the account that authored the note so client responses can render
// remote statuses as remote authors instead of as the local timeline owner.
func (u Timelines) HomeTimeline(ctx context.Context, account *models.Account, opts TimelineOptions) ([]TimelineItem, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	opts = normalizeTimelineOptions(opts)
	following, err := u.deps.FollowsRepo.ListFollowing(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	actors := make([]string, 0, len(following)+1)
	actors = append(actors, account.URI)
	for _, follow := range following {
		actors = append(actors, follow.RemoteActor)
	}
	notes, err := u.deps.NotesRepo.ListHomeTimelineNotesPaged(ctx, nil, account.ID, actors, opts.Limit, opts.MaxID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	items, derr := u.timelineItems(ctx, account, notes)
	if derr != nil {
		return nil, derr
	}
	boosts, err := u.deps.BoostsRepo.ListTimelineBoosts(ctx, nil, account.ID, opts.Limit, opts.MaxID)
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
func (u Timelines) PublicTimeline(ctx context.Context, account *models.Account, opts TimelineOptions) ([]TimelineItem, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	opts = normalizeTimelineOptions(opts)
	prefix := strings.TrimRight(u.deps.Host, "/") + "/users/"
	var notes []models.Note
	var err error
	switch {
	case opts.LocalOnly:
		notes, err = u.deps.NotesRepo.ListKnownLocalTimelineNotesPaged(ctx, nil, account.ID, prefix, opts.Limit, opts.MaxID)
	case opts.RemoteOnly:
		notes, err = u.deps.NotesRepo.ListKnownRemoteTimelineNotesPaged(ctx, nil, account.ID, prefix, opts.Limit, opts.MaxID)
	default:
		notes, err = u.deps.NotesRepo.ListKnownPublicTimelineNotesPaged(ctx, nil, account.ID, opts.Limit, opts.MaxID)
	}
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.timelineItems(ctx, account, notes)
}

func (u Timelines) timelineItems(ctx context.Context, localAccount *models.Account, notes []models.Note) ([]TimelineItem, *domainerrors.DomainError) {
	items := make([]TimelineItem, 0, len(notes))
	for _, note := range notes {
		author, derr := u.noteAuthor(ctx, localAccount, note)
		if derr != nil {
			return nil, derr
		}
		allowed, derr := u.accountDomainAllowed(ctx, author)
		if derr != nil {
			return nil, derr
		}
		if !allowed {
			continue
		}
		replyAccountID := u.replyAccountID(ctx, localAccount, note)
		media, err := u.deps.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
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

// accountDomainAllowed centralizes domain-block enforcement for timeline
// assembly. The use case asks the moderation port instead of inspecting storage.
func (u Timelines) accountDomainAllowed(ctx context.Context, account *models.Account) (bool, *domainerrors.DomainError) {
	if account.Domain == nil || *account.Domain == "" {
		return true, nil
	}
	blocked, err := u.deps.DomainBlocksRepo.DomainIsSuspended(ctx, nil, *account.Domain)
	if err != nil {
		return false, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	return !blocked, nil
}

// timelineItem enriches a stored Note with client API state such as boosts,
// favourites, bookmarks, pins, mentions, and media. It composes domain models
// through repository ports without leaking HTTP or database concerns.
func (u Timelines) timelineItem(
	ctx context.Context,
	localAccount *models.Account,
	note models.Note,
	author models.Account,
	replyAccountID *string,
	media []models.MediaAttachment,
) (*TimelineItem, *domainerrors.DomainError) {
	reblogsCount, err := u.deps.BoostsRepo.CountBoostsForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	reblogged, err := u.deps.BoostsRepo.BoostExists(ctx, nil, localAccount.ID, localAccount.URI, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	favouritesCount, err := u.deps.SocialRepo.CountInteractionsForNote(ctx, nil, note.ID, "favourite")
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	favourited, err := u.deps.SocialRepo.InteractionExists(ctx, nil, localAccount.ID, note.ID, "favourite")
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	bookmarked, err := u.deps.SocialRepo.InteractionExists(ctx, nil, localAccount.ID, note.ID, "bookmark")
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	pinned := false
	if author.UserID != nil && note.AttributedTo == author.URI {
		pinned, err = u.deps.SocialRepo.InteractionExists(ctx, nil, author.ID, note.ID, "pin")
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
	}

	mentions, err := u.deps.MentionsRepo.ListMentionsForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	poll, derr := u.pollForNote(ctx, localAccount, note)
	if derr != nil {
		return nil, derr
	}
	return &TimelineItem{
		ID:                 note.ID,
		URI:                note.URI,
		CreatedAt:          note.PublishedAt,
		Note:               note,
		Account:            author,
		InReplyToAccountID: replyAccountID,
		Media:              media,
		Mentions:           mentions,
		Poll:               poll,
		ReblogsCount:       reblogsCount,
		FavouritesCount:    favouritesCount,
		Reblogged:          reblogged,
		Favourited:         favourited,
		Bookmarked:         bookmarked,
		Pinned:             pinned,
	}, nil
}

func (u Timelines) boostTimelineItems(ctx context.Context, localAccount *models.Account, boosts []models.Boost) ([]TimelineItem, *domainerrors.DomainError) {
	items := make([]TimelineItem, 0, len(boosts))
	for _, boost := range boosts {
		item, ok, derr := u.boostTimelineItem(ctx, localAccount, boost)
		if derr != nil {
			return nil, derr
		}
		if ok {
			items = append(items, *item)
		}
	}
	return items, nil
}

// boostTimelineItem returns ok=false for intentionally skipped boosts, such as
// missing original notes or boosts involving suspended remote domains.
func (u Timelines) boostTimelineItem(ctx context.Context, localAccount *models.Account, boost models.Boost) (*TimelineItem, bool, *domainerrors.DomainError) {
	note, err := u.deps.NotesRepo.GetNoteByID(ctx, nil, boost.NoteID)
	if err != nil {
		return nil, false, nil
	}

	originalAuthor, derr := u.noteAuthor(ctx, localAccount, *note)
	if derr != nil {
		return nil, false, derr
	}
	allowed, derr := u.accountDomainAllowed(ctx, originalAuthor)
	if derr != nil || !allowed {
		return nil, false, derr
	}

	media, err := u.deps.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, false, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	inner, derr := u.timelineItem(ctx, localAccount, *note, *originalAuthor, u.replyAccountID(ctx, localAccount, *note), media)
	if derr != nil {
		return nil, false, derr
	}

	booster, derr := u.accountForActor(ctx, localAccount, boost.Actor)
	if derr != nil {
		return nil, false, derr
	}
	allowed, derr = u.accountDomainAllowed(ctx, booster)
	if derr != nil || !allowed {
		return nil, false, derr
	}

	item := &TimelineItem{
		ID:        boost.ID,
		URI:       boost.URI,
		CreatedAt: boost.PublishedAt,
		Note:      *note,
		Account:   *booster,
		Reblog:    inner,
	}
	return item, true, nil
}

func (u Timelines) accountForActor(ctx context.Context, localAccount *models.Account, actor string) (*models.Account, *domainerrors.DomainError) {
	if actor == localAccount.URI {
		return localAccount, nil
	}
	prefix := strings.TrimRight(u.deps.Host, "/") + "/users/"
	if strings.HasPrefix(actor, prefix) {
		account, err := u.deps.AccountsRepo.GetLocalAccountByUsername(ctx, nil, strings.TrimPrefix(actor, prefix))
		if err == nil {
			return account, nil
		}
	}
	remote, err := u.deps.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, actor)
	if err == nil {
		return u.ensureRemoteProfileImages(ctx, localAccount, u.refreshRemoteAccountIfStale(ctx, localAccount, remote)), nil
	}
	return fallbackRemoteAccount(actor), nil
}

const remoteAccountRefreshAfter = 24 * time.Hour

func (u Timelines) refreshRemoteAccountIfStale(ctx context.Context, signer *models.Account, cached *models.Account) *models.Account {
	if cached == nil || cached.URI == "" || u.deps.RemoteResolver == nil || time.Since(cached.FetchedAt) < remoteAccountRefreshAfter {
		return cached
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	fresh, err := u.deps.RemoteResolver.ResolveAccount(refreshCtx, cached.URI, signer)
	if err != nil || fresh == nil {
		return cached
	}
	updated, err := u.deps.RemoteAccountsRepo.UpsertRemoteAccount(ctx, nil, *fresh)
	cacheRemoteAccountProfileImagesAsync(u.deps.MediaRepo, u.deps.MediaStorage, u.deps.RemoteMediaFetcher, u.deps.RemoteAccountsRepo, u.deps.ProfileCacheNotifier, signer.ID, updated)
	if err != nil {
		return cached
	}
	return updated
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

func mergeTimelineItems(a, b []TimelineItem, limit int) []TimelineItem {
	items := append(a, b...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (u Timelines) replyAccountID(ctx context.Context, localAccount *models.Account, note models.Note) *string {
	if note.InReplyToID == nil || *note.InReplyToID == "" {
		return nil
	}
	parent, err := u.deps.NotesRepo.GetNoteByID(ctx, nil, *note.InReplyToID)
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

func (u Timelines) noteAuthor(ctx context.Context, localAccount *models.Account, note models.Note) (*models.Account, *domainerrors.DomainError) {
	if note.AttributedTo == "" || note.AttributedTo == localAccount.URI {
		return localAccount, nil
	}
	remote, err := u.deps.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, note.AttributedTo)
	if err == nil {
		return u.ensureRemoteProfileImages(ctx, localAccount, remote), nil
	}
	return fallbackRemoteAccount(note.AttributedTo), nil
}

func (u Timelines) ensureRemoteProfileImages(ctx context.Context, localAccount *models.Account, remote *models.Account) *models.Account {
	if localAccount == nil || remote == nil || remoteProfileImagesCached(remote) {
		return remote
	}
	cacheRemoteAccountProfileImagesAsync(u.deps.MediaRepo, u.deps.MediaStorage, u.deps.RemoteMediaFetcher, u.deps.RemoteAccountsRepo, u.deps.ProfileCacheNotifier, localAccount.ID, remote)
	return remote
}

func fallbackRemoteAccount(actor string) *models.Account {
	trimmed := strings.TrimRight(actor, "/")
	username := trimmed
	domain := ""
	if parts := strings.Split(trimmed, "/"); len(parts) > 0 {
		username = parts[len(parts)-1]
	}
	if strings.HasPrefix(actor, "http://") || strings.HasPrefix(actor, "https://") {
		withoutScheme := strings.TrimPrefix(strings.TrimPrefix(actor, "https://"), "http://")
		if host, _, ok := strings.Cut(withoutScheme, "/"); ok {
			domain = host
		}
	}
	return &models.Account{ID: AccountIDForRemoteActor(actor), Username: username, Domain: stringPtrValue(domain), DisplayName: stringPtrValue(username), URI: actor, URL: stringPtrValue(actor), ActorType: models.ActorTypePerson}
}

func stringPtrValue(value string) *string {
	return &value
}
