package clientapi

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
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

const remoteRepliesCacheTimeout = 20 * time.Second

var remoteRepliesCacheJobs sync.Map

func (u Statuses) GetStatus(ctx context.Context, localAccount *models.Account, statusID string) (*TimelineItem, *domainerrors.DomainError) {
	return u.getStatus(ctx, localAccount, statusID)
}

func (u Statuses) getStatus(ctx context.Context, localAccount *models.Account, statusID string) (*TimelineItem, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	note, err := u.deps.NotesRepo.GetNoteByID(ctx, nil, statusID)
	if err != nil || note.LocalAccountID != localAccount.ID {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "status not found")
	}
	author, derr := u.noteAuthor(ctx, localAccount, *note)
	if derr != nil {
		return nil, derr
	}
	if author.Domain != nil && *author.Domain != "" {
		blocked, err := u.deps.DomainBlocksRepo.DomainIsSuspended(ctx, nil, *author.Domain)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		if blocked {
			return nil, domainerrors.New(domainerrors.ErrNotFound, "status not found")
		}
	}
	media, err := u.deps.MediaRepo.ListMediaForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return u.timelineItem(ctx, localAccount, *note, *author, u.replyAccountID(ctx, localAccount, *note), media)
}

func (u Statuses) DeleteStatus(ctx context.Context, localAccount *models.Account, statusID string) (*DeleteStatusResult, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	deleteID, err := u.deps.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res, derr := u.deps.DeleteObjectUC.DeleteObject(ctx, apUsecases.DeleteObjectInput{Username: localAccount.Username, ObjectID: statusID, DeleteID: deleteID})
	if derr != nil {
		if derr.Code == domainerrors.ErrNotFound {
			return nil, domainerrors.New(domainerrors.ErrNotFound, "status not found")
		}
		return nil, derr
	}
	if derr := u.cleanupUnreferencedMedia(ctx, res.Media); derr != nil {
		return nil, derr
	}
	return &DeleteStatusResult{Account: res.Account, RawJSON: res.RawJSON, FollowerInboxes: res.FollowerInboxes}, nil
}

func (u Statuses) StatusContext(ctx context.Context, localAccount *models.Account, statusID string) (*StatusContext, *domainerrors.DomainError) {
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

func (u Statuses) statusDescendants(ctx context.Context, localAccount *models.Account, note models.Note, depth int, seen map[string]bool) ([]TimelineItem, *domainerrors.DomainError) {
	if depth >= 40 {
		return []TimelineItem{}, nil
	}
	if depth == 0 {
		u.cacheRemoteRepliesNow(ctx, localAccount, note)
	} else {
		u.cacheRemoteReplies(ctx, localAccount, note)
	}
	replies, err := u.deps.NotesRepo.ListReplies(ctx, nil, localAccount.ID, note.ID, note.URI)
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

func (u Statuses) statusAncestors(ctx context.Context, localAccount *models.Account, note models.Note) ([]TimelineItem, []string, *domainerrors.DomainError) {
	return u.statusAncestorsWithDepth(ctx, localAccount, note, 0, map[string]bool{note.ID: true})
}

func (u Statuses) statusAncestorsWithDepth(ctx context.Context, localAccount *models.Account, note models.Note, depth int, seen map[string]bool) ([]TimelineItem, []string, *domainerrors.DomainError) {
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
	media, err := u.deps.MediaRepo.ListMediaForNote(ctx, nil, parent.ID)
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

func (u Statuses) parentNote(ctx context.Context, localAccount *models.Account, note models.Note) (*models.Note, string, *domainerrors.DomainError) {
	if note.InReplyToID != nil && *note.InReplyToID != "" {
		parent, err := u.deps.NotesRepo.GetNoteByID(ctx, nil, *note.InReplyToID)
		if err != nil {
			return nil, "", nil
		}
		return parent, "", nil
	}
	if note.InReplyToURI == nil || *note.InReplyToURI == "" {
		return nil, "", nil
	}
	parent, err := u.deps.NotesRepo.GetNoteByURI(ctx, nil, *note.InReplyToURI)
	if err == nil {
		return parent, "", nil
	}
	if err != sql.ErrNoRows {
		return nil, "", domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if err := u.cacheRemoteContextNote(ctx, localAccount, *note.InReplyToURI); err != nil {
		return nil, fmt.Sprintf("Could not load parent post %s: %v", *note.InReplyToURI, err), nil
	}
	parent, err = u.deps.NotesRepo.GetNoteByURI(ctx, nil, *note.InReplyToURI)
	if err != nil {
		return nil, fmt.Sprintf("Could not load parent post %s", *note.InReplyToURI), nil
	}
	return parent, "", nil
}

func (u Statuses) cacheRemoteContextNote(ctx context.Context, localAccount *models.Account, objectURI string) error {
	return u.deps.HydrateRemoteObjectUC.HydrateRemoteObject(ctx, *localAccount, objectURI)
}

func (u Statuses) cacheRemoteRepliesNow(ctx context.Context, localAccount *models.Account, note models.Note) {
	if note.URI == "" || localAccount == nil || note.AttributedTo == localAccount.URI || !(strings.HasPrefix(note.URI, "https://") || strings.HasPrefix(note.URI, "http://")) {
		return
	}
	key := localAccount.ID + "\x00" + note.URI
	if _, loaded := remoteRepliesCacheJobs.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	defer remoteRepliesCacheJobs.Delete(key)
	cacheCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	_ = u.deps.HydrateRemoteObjectUC.HydrateRemoteReplies(cacheCtx, *localAccount, note.URI)
}

func (u Statuses) cacheRemoteReplies(ctx context.Context, localAccount *models.Account, note models.Note) {
	if note.URI == "" || localAccount == nil || note.AttributedTo == localAccount.URI || !(strings.HasPrefix(note.URI, "https://") || strings.HasPrefix(note.URI, "http://")) {
		return
	}
	key := localAccount.ID + "\x00" + note.URI
	if _, loaded := remoteRepliesCacheJobs.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	account := *localAccount
	go func() {
		defer remoteRepliesCacheJobs.Delete(key)
		cacheCtx, cancel := context.WithTimeout(context.Background(), remoteRepliesCacheTimeout)
		defer cancel()
		_ = u.deps.HydrateRemoteObjectUC.HydrateRemoteReplies(cacheCtx, account, note.URI)
	}()
}
