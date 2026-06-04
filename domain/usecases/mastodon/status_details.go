package mastodon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

type DeleteStatusResult struct {
	Account         models.Account
	RawJSON         []byte
	FollowerInboxes []string
}

type StatusContext struct {
	Ancestors   []TimelineItem
	Descendants []TimelineItem
	Warnings    []string
}

func (u UseCase) GetStatus(ctx context.Context, localAccount *models.Account, statusID string) (*TimelineItem, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	note, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, statusID)
	if err != nil || note.LocalAccountID != localAccount.ID {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "status not found")
	}
	author, derr := u.noteAuthor(ctx, localAccount, *note)
	if derr != nil {
		return nil, derr
	}
	if author.Domain != nil && *author.Domain != "" {
		blocked, err := u.cfg.DomainBlocksRepo.DomainIsSuspended(ctx, nil, *author.Domain)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		if blocked {
			return nil, domainerrors.New(domainerrors.ErrNotFound, "status not found")
		}
	}
	media, err := u.cfg.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.timelineItem(ctx, localAccount, *note, *author, u.replyAccountID(ctx, localAccount, *note), media)
}

func (u UseCase) DeleteStatus(ctx context.Context, localAccount *models.Account, statusID string) (*DeleteStatusResult, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	note, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, statusID)
	if err != nil || note.LocalAccountID != localAccount.ID || note.AttributedTo != localAccount.URI {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "status not found")
	}
	media, err := u.cfg.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if err := u.cfg.NotesRepo.DeleteNoteByID(ctx, nil, statusID); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if derr := u.cleanupUnreferencedMedia(ctx, media); derr != nil {
		return nil, derr
	}
	deleteID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	raw, err := json.Marshal(map[string]any{activityStreamsContextKey: activityStreamsContextURI, "id": localAccount.URI + "/deletes/" + deleteID, "type": "Delete", "actor": localAccount.URI, "object": note.URI})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	followers, err := u.cfg.FollowsRepo.ListFollowers(ctx, nil, localAccount.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	inboxes := make([]string, 0, len(followers))
	for _, follower := range followers {
		if follower.RemoteInbox != nil {
			inboxes = append(inboxes, *follower.RemoteInbox)
		}
	}
	return &DeleteStatusResult{Account: *localAccount, RawJSON: raw, FollowerInboxes: inboxes}, nil
}

func (u UseCase) StatusContext(ctx context.Context, localAccount *models.Account, statusID string) (*StatusContext, *domainerrors.DomainError) {
	item, derr := u.GetStatus(ctx, localAccount, statusID)
	if derr != nil {
		return nil, derr
	}
	ancestors, warnings, derr := u.statusAncestors(ctx, localAccount, item.Note)
	if derr != nil {
		return nil, derr
	}
	seen := map[string]bool{item.Note.ID: true}
	for _, ancestor := range ancestors {
		seen[ancestor.Note.ID] = true
	}
	descendants, derr := u.statusDescendants(ctx, localAccount, item.Note, 0, seen)
	if derr != nil {
		return nil, derr
	}
	return &StatusContext{Ancestors: uniqueTimelineItems(ancestors), Descendants: uniqueTimelineItems(descendants), Warnings: warnings}, nil
}

func (u UseCase) statusDescendants(ctx context.Context, localAccount *models.Account, note models.Note, depth int, seen map[string]bool) ([]TimelineItem, *domainerrors.DomainError) {
	if depth >= 40 {
		return []TimelineItem{}, nil
	}
	replies, err := u.cfg.NotesRepo.ListReplies(ctx, nil, localAccount.ID, note.ID, note.URI)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	descendants := make([]TimelineItem, 0, len(replies))
	for _, reply := range replies {
		if seen[reply.ID] {
			continue
		}
		seen[reply.ID] = true
		items, derr := u.timelineItems(ctx, localAccount, []models.Note{reply})
		if derr != nil {
			return nil, derr
		}
		descendants = append(descendants, items...)
		childItems, derr := u.statusDescendants(ctx, localAccount, reply, depth+1, seen)
		if derr != nil {
			return nil, derr
		}
		descendants = append(descendants, childItems...)
	}
	return descendants, nil
}

func (u UseCase) statusAncestors(ctx context.Context, localAccount *models.Account, note models.Note) ([]TimelineItem, []string, *domainerrors.DomainError) {
	return u.statusAncestorsWithDepth(ctx, localAccount, note, 0, map[string]bool{note.ID: true})
}

func (u UseCase) statusAncestorsWithDepth(ctx context.Context, localAccount *models.Account, note models.Note, depth int, seen map[string]bool) ([]TimelineItem, []string, *domainerrors.DomainError) {
	if depth >= 40 {
		return []TimelineItem{}, nil, nil
	}
	parent, warning, derr := u.parentNote(ctx, localAccount, note)
	if derr != nil {
		return nil, nil, derr
	}
	if warning != "" {
		return []TimelineItem{}, []string{warning}, nil
	}
	if parent == nil || seen[parent.ID] {
		return []TimelineItem{}, nil, nil
	}
	seen[parent.ID] = true
	items, warnings, derr := u.statusAncestorsWithDepth(ctx, localAccount, *parent, depth+1, seen)
	if derr != nil {
		return nil, nil, derr
	}
	author, derr := u.noteAuthor(ctx, localAccount, *parent)
	if derr != nil {
		return nil, nil, derr
	}
	media, err := u.cfg.MediaRepo.ListMediaForNote(ctx, nil, parent.ID)
	if err != nil {
		return nil, nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	item, derr := u.timelineItem(ctx, localAccount, *parent, *author, u.replyAccountID(ctx, localAccount, *parent), media)
	if derr != nil {
		return nil, nil, derr
	}
	items = append(items, *item)
	return items, warnings, nil
}

func uniqueTimelineItems(items []TimelineItem) []TimelineItem {
	seen := make(map[string]bool, len(items))
	unique := make([]TimelineItem, 0, len(items))
	for _, item := range items {
		id := item.Note.ID
		if id == "" {
			id = item.ID
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, item)
	}
	return unique
}

func (u UseCase) parentNote(ctx context.Context, localAccount *models.Account, note models.Note) (*models.Note, string, *domainerrors.DomainError) {
	if note.InReplyToID != nil && *note.InReplyToID != "" {
		parent, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, *note.InReplyToID)
		if err != nil {
			return nil, "", nil
		}
		return parent, "", nil
	}
	if note.InReplyToURI == nil || *note.InReplyToURI == "" {
		return nil, "", nil
	}
	parent, err := u.cfg.NotesRepo.GetNoteByURI(ctx, nil, *note.InReplyToURI)
	if err == nil {
		return parent, "", nil
	}
	if err != sql.ErrNoRows {
		return nil, "", domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if err := u.cacheRemoteContextNote(ctx, localAccount, *note.InReplyToURI); err != nil {
		return nil, fmt.Sprintf("Could not load parent post %s: %v", *note.InReplyToURI, err), nil
	}
	parent, err = u.cfg.NotesRepo.GetNoteByURI(ctx, nil, *note.InReplyToURI)
	if err != nil {
		return nil, fmt.Sprintf("Could not load parent post %s", *note.InReplyToURI), nil
	}
	return parent, "", nil
}

func (u UseCase) cacheRemoteContextNote(ctx context.Context, localAccount *models.Account, objectURI string) error {
	raw, err := u.cfg.RemoteObjectFetcher.FetchObject(ctx, objectURI, localAccount)
	if err != nil {
		return err
	}
	note, ok := apUsecases.ExtractNote(raw)
	if !ok {
		note, ok = apUsecases.ExtractStandaloneNote(raw)
	}
	if !ok || note.URI == "" || note.Visibility == "direct" {
		return nil
	}
	replyID, replyURI := u.remoteReplyIDs(ctx, localAccount, note)
	return u.cfg.TxProvider.RunInTx(ctx, nil, func(ctx context.Context, tx db.Tx) error {
		if _, err := u.cfg.NotesRepo.GetNoteByURI(ctx, &tx, note.URI); err == nil {
			return nil
		} else if err != sql.ErrNoRows {
			return err
		}
		activity, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: localAccount.ID, Direction: models.ActivityDirectionInbox, Type: "Create", Actor: note.AttributedTo, Object: note.URI, RawJSON: string(raw)})
		if err != nil {
			return err
		}
		_, err = u.cfg.NotesRepo.CreateNote(ctx, &tx, repos.CreateNoteInput{LocalAccountID: localAccount.ID, ActivityID: activity.ID, URI: note.URI, Content: u.cfg.ContentSanitizer.SanitizeHTML(note.Content), PlainText: u.cfg.ContentSanitizer.StripHTMLFromText(note.Content), Visibility: note.Visibility, Sensitive: note.Sensitive, SpoilerText: note.SpoilerText, AttributedTo: note.AttributedTo, InReplyToID: replyID, InReplyToURI: replyURI, PublishedAt: note.PublishedAt})
		return err
	})
}
