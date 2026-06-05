package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type mediaCleaner struct {
	mediaRepo    repos.MediaRepository
	mediaStorage ports.MediaStorage
}

func (c mediaCleaner) cleanupUnreferencedMedia(ctx context.Context, media []models.MediaAttachment) *domainerrors.DomainError {
	for _, item := range media {
		attached, err := c.mediaRepo.MediaAttachmentIsAttached(ctx, nil, item.ID)
		if err != nil {
			return domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		if attached {
			continue
		}
		if derr := c.deleteMediaFilesAndRow(ctx, item); derr != nil {
			return derr
		}
	}
	return nil
}

func (c mediaCleaner) deleteMediaFilesAndRow(ctx context.Context, media models.MediaAttachment) *domainerrors.DomainError {
	if media.StoragePath != "" {
		if err := c.mediaStorage.DeleteMedia(ctx, media.StoragePath); err != nil {
			return domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
	}
	if err := c.mediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return nil
}

func (u Statuses) cleanupUnreferencedMedia(ctx context.Context, media []models.MediaAttachment) *domainerrors.DomainError {
	return (mediaCleaner{mediaRepo: u.deps.MediaRepo, mediaStorage: u.deps.MediaStorage}).cleanupUnreferencedMedia(ctx, media)
}

func (u Profile) deleteMediaFilesAndRow(ctx context.Context, media models.MediaAttachment) *domainerrors.DomainError {
	return (mediaCleaner{mediaRepo: u.deps.MediaRepo, mediaStorage: u.deps.MediaStorage}).deleteMediaFilesAndRow(ctx, media)
}
