package mastodon

import (
	"context"
	"encoding/json"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

type DeleteStatusResult struct {
	Account         models.Account
	RawJSON         []byte
	FollowerInboxes []string
}

type StatusContext struct {
	Ancestors   []TimelineItem
	Descendants []TimelineItem
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
	raw, err := json.Marshal(map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": localAccount.URI + "/deletes/" + deleteID, "type": "Delete", "actor": localAccount.URI, "object": note.URI})
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
	ancestors, derr := u.statusAncestors(ctx, localAccount, item.Note)
	if derr != nil {
		return nil, derr
	}
	replies, err := u.cfg.NotesRepo.ListReplies(ctx, nil, localAccount.ID, statusID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	descendants, derr := u.timelineItems(ctx, localAccount, replies)
	if derr != nil {
		return nil, derr
	}
	return &StatusContext{Ancestors: ancestors, Descendants: descendants}, nil
}

func (u UseCase) statusAncestors(ctx context.Context, localAccount *models.Account, note models.Note) ([]TimelineItem, *domainerrors.DomainError) {
	if note.InReplyToID == nil || *note.InReplyToID == "" {
		return []TimelineItem{}, nil
	}
	parent, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, *note.InReplyToID)
	if err != nil {
		return []TimelineItem{}, nil
	}
	items, derr := u.statusAncestors(ctx, localAccount, *parent)
	if derr != nil {
		return nil, derr
	}
	author, derr := u.noteAuthor(ctx, localAccount, *parent)
	if derr != nil {
		return nil, derr
	}
	media, err := u.cfg.MediaRepo.ListMediaForNote(ctx, nil, parent.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	item, derr := u.timelineItem(ctx, localAccount, *parent, *author, u.replyAccountID(ctx, localAccount, *parent), media)
	if derr != nil {
		return nil, derr
	}
	items = append(items, *item)
	return items, nil
}
