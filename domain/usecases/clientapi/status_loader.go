package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type statusLoader struct {
	notesRepo        repos.NotesRepository
	mediaRepo        repos.MediaRepository
	domainBlocksRepo repos.DomainBlocksRepository
	timeline         timelineBuilder
}

func statusLoaderFromInteractions(cfg InteractionsConfig) statusLoader {
	return statusLoader{notesRepo: cfg.NotesRepo, mediaRepo: cfg.MediaRepo, domainBlocksRepo: cfg.DomainBlocksRepo, timeline: timelineBuilderFromInteractions(cfg)}
}

func statusLoaderFromNotifications(cfg NotificationsConfig) statusLoader {
	return statusLoader{notesRepo: cfg.NotesRepo, mediaRepo: cfg.MediaRepo, domainBlocksRepo: cfg.DomainBlocksRepo, timeline: timelineBuilderFromNotifications(cfg)}
}

func statusLoaderFromConversations(cfg ConversationsConfig) statusLoader {
	return statusLoader{notesRepo: cfg.NotesRepo, mediaRepo: cfg.MediaRepo, domainBlocksRepo: cfg.DomainBlocksRepo, timeline: timelineBuilderFromConversations(cfg)}
}

func (l statusLoader) getStatus(ctx context.Context, localAccount *models.Account, statusID string) (*TimelineItem, *domainerrors.DomainError) {
	if derr := requireAccount(localAccount); derr != nil {
		return nil, derr
	}
	note, err := l.notesRepo.GetNoteByID(ctx, nil, statusID)
	if err != nil || note.LocalAccountID != localAccount.ID {
		return nil, domainerrors.New(domainerrors.ErrNotFound, statusNotFoundMessage)
	}
	author, derr := l.timeline.noteAuthor(ctx, localAccount, *note)
	if derr != nil {
		return nil, derr
	}
	if author.Domain != nil && *author.Domain != "" {
		blocked, err := l.domainBlocksRepo.DomainIsSuspended(ctx, nil, *author.Domain)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		if blocked {
			return nil, domainerrors.New(domainerrors.ErrNotFound, statusNotFoundMessage)
		}
	}
	media, err := l.mediaRepo.ListMediaForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return l.timeline.timelineItem(ctx, localAccount, *note, *author, l.timeline.replyAccountID(ctx, localAccount, *note), media)
}

func (u Interactions) getStatus(ctx context.Context, localAccount *models.Account, statusID string) (*TimelineItem, *domainerrors.DomainError) {
	return statusLoaderFromInteractions(u.deps).getStatus(ctx, localAccount, statusID)
}

func (u Notifications) getStatus(ctx context.Context, localAccount *models.Account, statusID string) (*TimelineItem, *domainerrors.DomainError) {
	return statusLoaderFromNotifications(u.deps).getStatus(ctx, localAccount, statusID)
}

func (u Conversations) getStatus(ctx context.Context, localAccount *models.Account, statusID string) (*TimelineItem, *domainerrors.DomainError) {
	return statusLoaderFromConversations(u.deps).getStatus(ctx, localAccount, statusID)
}
