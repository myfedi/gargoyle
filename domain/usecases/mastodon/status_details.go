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
	return &TimelineItem{Note: *note, Account: *author}, nil
}

func (u UseCase) DeleteStatus(ctx context.Context, localAccount *models.Account, statusID string) (*DeleteStatusResult, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	note, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, statusID)
	if err != nil || note.LocalAccountID != localAccount.ID || note.AttributedTo != localAccount.URI {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "status not found")
	}
	if err := u.cfg.NotesRepo.DeleteNoteByID(ctx, nil, statusID); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
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
	if _, derr := u.GetStatus(ctx, localAccount, statusID); derr != nil {
		return nil, derr
	}
	return &StatusContext{Ancestors: []TimelineItem{}, Descendants: []TimelineItem{}}, nil
}
