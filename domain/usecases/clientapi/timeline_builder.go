package clientapi

import (
	"context"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type timelineBuilder struct {
	host               string
	notesRepo          repos.NotesRepository
	accountsRepo       repos.AccountsRepo
	mediaRepo          repos.MediaRepository
	socialRepo         repos.SocialRepository
	boostsRepo         repos.BoostsRepository
	mentionsRepo       repos.MentionsRepository
	pollsRepo          repos.PollsRepository
	remoteAccountsRepo repos.RemoteAccountsRepository
	domainBlocksRepo   repos.DomainBlocksRepository
	remoteResolver     RemoteAccountResolver
}

func timelineBuilderFromAccounts(cfg AccountsConfig) timelineBuilder {
	return timelineBuilder{host: cfg.Host, notesRepo: cfg.NotesRepo, accountsRepo: cfg.AccountsRepo, mediaRepo: cfg.MediaRepo, socialRepo: cfg.SocialRepo, boostsRepo: cfg.BoostsRepo, mentionsRepo: cfg.MentionsRepo, pollsRepo: cfg.PollsRepo, remoteAccountsRepo: cfg.RemoteAccountsRepo, domainBlocksRepo: cfg.DomainBlocksRepo, remoteResolver: cfg.RemoteResolver}
}

func timelineBuilderFromStatuses(cfg StatusesConfig) timelineBuilder {
	return timelineBuilder{host: cfg.Host, notesRepo: cfg.NotesRepo, accountsRepo: cfg.AccountsRepo, mediaRepo: cfg.MediaRepo, socialRepo: cfg.SocialRepo, boostsRepo: cfg.BoostsRepo, mentionsRepo: cfg.MentionsRepo, pollsRepo: cfg.PollsRepo, remoteAccountsRepo: cfg.RemoteAccountsRepo, domainBlocksRepo: cfg.DomainBlocksRepo, remoteResolver: cfg.RemoteResolver}
}

func timelineBuilderFromInteractions(cfg InteractionsConfig) timelineBuilder {
	return timelineBuilder{host: cfg.Host, notesRepo: cfg.NotesRepo, accountsRepo: cfg.AccountsRepo, mediaRepo: cfg.MediaRepo, socialRepo: cfg.SocialRepo, boostsRepo: cfg.BoostsRepo, mentionsRepo: cfg.MentionsRepo, pollsRepo: cfg.PollsRepo, remoteAccountsRepo: cfg.RemoteAccountsRepo, domainBlocksRepo: cfg.DomainBlocksRepo, remoteResolver: cfg.RemoteResolver}
}

func timelineBuilderFromNotifications(cfg NotificationsConfig) timelineBuilder {
	return timelineBuilder{host: cfg.Host, notesRepo: cfg.NotesRepo, accountsRepo: cfg.AccountsRepo, mediaRepo: cfg.MediaRepo, socialRepo: cfg.SocialRepo, boostsRepo: cfg.BoostsRepo, mentionsRepo: cfg.MentionsRepo, pollsRepo: cfg.PollsRepo, remoteAccountsRepo: cfg.RemoteAccountsRepo, domainBlocksRepo: cfg.DomainBlocksRepo, remoteResolver: cfg.RemoteResolver}
}

func timelineBuilderFromConversations(cfg ConversationsConfig) timelineBuilder {
	return timelineBuilder{host: cfg.Host, notesRepo: cfg.NotesRepo, accountsRepo: cfg.AccountsRepo, mediaRepo: cfg.MediaRepo, socialRepo: cfg.SocialRepo, boostsRepo: cfg.BoostsRepo, mentionsRepo: cfg.MentionsRepo, pollsRepo: cfg.PollsRepo, remoteAccountsRepo: cfg.RemoteAccountsRepo, domainBlocksRepo: cfg.DomainBlocksRepo, remoteResolver: cfg.RemoteResolver}
}

func (b timelineBuilder) timelineItem(ctx context.Context, localAccount *models.Account, note models.Note, author models.Account, replyAccountID *string, media []models.MediaAttachment) (*TimelineItem, *domainerrors.DomainError) {
	reblogsCount, err := b.boostsRepo.CountBoostsForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	reblogged, err := b.boostsRepo.BoostExists(ctx, nil, localAccount.ID, localAccount.URI, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	favouritesCount, err := b.socialRepo.CountInteractionsForNote(ctx, nil, note.ID, "favourite")
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	favourited, err := b.socialRepo.InteractionExists(ctx, nil, localAccount.ID, note.ID, "favourite")
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	bookmarked, err := b.socialRepo.InteractionExists(ctx, nil, localAccount.ID, note.ID, "bookmark")
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	pinned := false
	if author.UserID != nil && note.AttributedTo == author.URI {
		pinned, err = b.socialRepo.InteractionExists(ctx, nil, author.ID, note.ID, "pin")
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
	}
	mentions, err := b.mentionsRepo.ListMentionsForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	poll, derr := b.pollForNote(ctx, localAccount, note)
	if derr != nil {
		return nil, derr
	}
	return &TimelineItem{ID: note.ID, URI: note.URI, CreatedAt: note.PublishedAt, Note: note, Account: author, InReplyToAccountID: replyAccountID, Media: media, Mentions: mentions, Poll: poll, ReblogsCount: reblogsCount, FavouritesCount: favouritesCount, Reblogged: reblogged, Favourited: favourited, Bookmarked: bookmarked, Pinned: pinned}, nil
}

func (b timelineBuilder) timelineItems(ctx context.Context, localAccount *models.Account, notes []models.Note) ([]TimelineItem, *domainerrors.DomainError) {
	items := make([]TimelineItem, 0, len(notes))
	for _, note := range notes {
		author, derr := b.noteAuthor(ctx, localAccount, note)
		if derr != nil {
			return nil, derr
		}
		allowed, derr := b.accountDomainAllowed(ctx, author)
		if derr != nil {
			return nil, derr
		}
		if !allowed {
			continue
		}
		replyAccountID := b.replyAccountID(ctx, localAccount, note)
		media, err := b.mediaRepo.ListMediaForNote(ctx, nil, note.ID)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		item, derr := b.timelineItem(ctx, localAccount, note, *author, replyAccountID, media)
		if derr != nil {
			return nil, derr
		}
		items = append(items, *item)
	}
	return items, nil
}

func (b timelineBuilder) boostTimelineItems(ctx context.Context, localAccount *models.Account, boosts []models.Boost) ([]TimelineItem, *domainerrors.DomainError) {
	items := make([]TimelineItem, 0, len(boosts))
	for _, boost := range boosts {
		item, ok, derr := b.boostTimelineItem(ctx, localAccount, boost)
		if derr != nil {
			return nil, derr
		}
		if ok {
			items = append(items, *item)
		}
	}
	return items, nil
}

func (b timelineBuilder) boostTimelineItem(ctx context.Context, localAccount *models.Account, boost models.Boost) (*TimelineItem, bool, *domainerrors.DomainError) {
	note, err := b.notesRepo.GetNoteByID(ctx, nil, boost.NoteID)
	if err != nil {
		return nil, false, nil
	}
	originalAuthor, derr := b.noteAuthor(ctx, localAccount, *note)
	if derr != nil {
		return nil, false, derr
	}
	allowed, derr := b.accountDomainAllowed(ctx, originalAuthor)
	if derr != nil || !allowed {
		return nil, false, derr
	}
	media, err := b.mediaRepo.ListMediaForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, false, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	inner, derr := b.timelineItem(ctx, localAccount, *note, *originalAuthor, b.replyAccountID(ctx, localAccount, *note), media)
	if derr != nil {
		return nil, false, derr
	}
	booster, derr := b.accountForActor(ctx, localAccount, boost.Actor)
	if derr != nil {
		return nil, false, derr
	}
	allowed, derr = b.accountDomainAllowed(ctx, booster)
	if derr != nil || !allowed {
		return nil, false, derr
	}
	return &TimelineItem{ID: boost.ID, URI: boost.URI, CreatedAt: boost.PublishedAt, Note: *note, Account: *booster, Reblog: inner}, true, nil
}

func (b timelineBuilder) replyAccountID(ctx context.Context, localAccount *models.Account, note models.Note) *string {
	if note.InReplyToID == nil || *note.InReplyToID == "" {
		return nil
	}
	parent, err := b.notesRepo.GetNoteByID(ctx, nil, *note.InReplyToID)
	if err != nil {
		return nil
	}
	author, derr := b.noteAuthor(ctx, localAccount, *parent)
	if derr != nil {
		return nil
	}
	return &author.ID
}

func (b timelineBuilder) noteAuthor(ctx context.Context, localAccount *models.Account, note models.Note) (*models.Account, *domainerrors.DomainError) {
	if note.AttributedTo == "" || note.AttributedTo == localAccount.URI {
		return localAccount, nil
	}
	remote, err := b.remoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, note.AttributedTo)
	if err == nil {
		return remote, nil
	}
	return fallbackRemoteAccount(note.AttributedTo), nil
}

func (b timelineBuilder) accountForActor(ctx context.Context, localAccount *models.Account, actor string) (*models.Account, *domainerrors.DomainError) {
	if actor == localAccount.URI {
		return localAccount, nil
	}
	prefix := strings.TrimRight(b.host, "/") + "/users/"
	if strings.HasPrefix(actor, prefix) {
		account, err := b.accountsRepo.GetLocalAccountByUsername(ctx, nil, strings.TrimPrefix(actor, prefix))
		if err == nil {
			return account, nil
		}
	}
	remote, err := b.remoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, actor)
	if err == nil {
		return b.refreshRemoteAccountIfStale(ctx, localAccount, remote), nil
	}
	return fallbackRemoteAccount(actor), nil
}

func (b timelineBuilder) refreshRemoteAccountIfStale(ctx context.Context, signer *models.Account, cached *models.Account) *models.Account {
	if cached == nil || cached.URI == "" || b.remoteResolver == nil || time.Since(cached.FetchedAt) < remoteAccountRefreshAfter {
		return cached
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	fresh, err := b.remoteResolver.ResolveAccount(refreshCtx, cached.URI, signer)
	if err != nil || fresh == nil {
		return cached
	}
	updated, err := b.remoteAccountsRepo.UpsertRemoteAccount(ctx, nil, *fresh)
	if err != nil {
		return cached
	}
	return updated
}

func (b timelineBuilder) accountDomainAllowed(ctx context.Context, account *models.Account) (bool, *domainerrors.DomainError) {
	if account.Domain == nil || *account.Domain == "" {
		return true, nil
	}
	blocked, err := b.domainBlocksRepo.DomainIsSuspended(ctx, nil, *account.Domain)
	if err != nil {
		return false, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return !blocked, nil
}

func (b timelineBuilder) ensureRemoteDomainAllowed(ctx context.Context, actorOrURL string) *domainerrors.DomainError {
	return ensureRemoteDomainAllowed(ctx, b.domainBlocksRepo, actorOrURL)
}

func (b timelineBuilder) pollForNote(ctx context.Context, account *models.Account, note models.Note) (*models.Poll, *domainerrors.DomainError) {
	if note.ObjectType != "Question" {
		return nil, nil
	}
	options, err := b.pollsRepo.GetPollOptions(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if len(options) == 0 {
		return nil, nil
	}
	ownVotes, err := b.pollsRepo.LocalVoteChoices(ctx, nil, note.ID, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &models.Poll{NoteID: note.ID, Options: options, Multiple: note.PollMultiple, ExpiresAt: note.PollExpiresAt, Voted: len(ownVotes) > 0, OwnVotes: ownVotes}, nil
}

func (u Accounts) timelineItem(ctx context.Context, localAccount *models.Account, note models.Note, author models.Account, replyAccountID *string, media []models.MediaAttachment) (*TimelineItem, *domainerrors.DomainError) {
	return timelineBuilderFromAccounts(u.deps).timelineItem(ctx, localAccount, note, author, replyAccountID, media)
}

func (u Accounts) boostTimelineItems(ctx context.Context, localAccount *models.Account, boosts []models.Boost) ([]TimelineItem, *domainerrors.DomainError) {
	return timelineBuilderFromAccounts(u.deps).boostTimelineItems(ctx, localAccount, boosts)
}

func (u Accounts) replyAccountID(ctx context.Context, localAccount *models.Account, note models.Note) *string {
	return timelineBuilderFromAccounts(u.deps).replyAccountID(ctx, localAccount, note)
}

func (u Accounts) ensureRemoteDomainAllowed(ctx context.Context, actorOrURL string) *domainerrors.DomainError {
	return ensureRemoteDomainAllowed(ctx, u.deps.DomainBlocksRepo, actorOrURL)
}

func (u Statuses) timelineItem(ctx context.Context, localAccount *models.Account, note models.Note, author models.Account, replyAccountID *string, media []models.MediaAttachment) (*TimelineItem, *domainerrors.DomainError) {
	return timelineBuilderFromStatuses(u.deps).timelineItem(ctx, localAccount, note, author, replyAccountID, media)
}

func (u Statuses) timelineItems(ctx context.Context, localAccount *models.Account, notes []models.Note) ([]TimelineItem, *domainerrors.DomainError) {
	return timelineBuilderFromStatuses(u.deps).timelineItems(ctx, localAccount, notes)
}

func (u Statuses) replyAccountID(ctx context.Context, localAccount *models.Account, note models.Note) *string {
	return timelineBuilderFromStatuses(u.deps).replyAccountID(ctx, localAccount, note)
}

func (u Statuses) noteAuthor(ctx context.Context, localAccount *models.Account, note models.Note) (*models.Account, *domainerrors.DomainError) {
	return timelineBuilderFromStatuses(u.deps).noteAuthor(ctx, localAccount, note)
}

func (u Statuses) ensureRemoteDomainAllowed(ctx context.Context, actorOrURL string) *domainerrors.DomainError {
	return ensureRemoteDomainAllowed(ctx, u.deps.DomainBlocksRepo, actorOrURL)
}

func (u Interactions) timelineItem(ctx context.Context, localAccount *models.Account, note models.Note, author models.Account, replyAccountID *string, media []models.MediaAttachment) (*TimelineItem, *domainerrors.DomainError) {
	return timelineBuilderFromInteractions(u.deps).timelineItem(ctx, localAccount, note, author, replyAccountID, media)
}

func (u Interactions) replyAccountID(ctx context.Context, localAccount *models.Account, note models.Note) *string {
	return timelineBuilderFromInteractions(u.deps).replyAccountID(ctx, localAccount, note)
}

func (u Conversations) accountForActor(ctx context.Context, localAccount *models.Account, actor string) (*models.Account, *domainerrors.DomainError) {
	return timelineBuilderFromConversations(u.deps).accountForActor(ctx, localAccount, actor)
}

func (u Timelines) pollForNote(ctx context.Context, account *models.Account, note models.Note) (*models.Poll, *domainerrors.DomainError) {
	return (timelineBuilder{pollsRepo: u.deps.PollsRepo}).pollForNote(ctx, account, note)
}
