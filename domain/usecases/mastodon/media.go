package mastodon

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type UploadMediaInput struct {
	FileName    string
	ContentType string
	Data        []byte
	Description string
}

type UpdateMediaInput struct {
	Description string
}

type CleanupMediaResult struct {
	DeletedUnattached int
	DeletedBroken     int
}

func (u UseCase) UploadMedia(ctx context.Context, account *models.Account, input UploadMediaInput) (*models.MediaAttachment, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	if len(input.Data) == 0 {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "media file is required")
	}
	if input.ContentType == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "media content type is required")
	}
	id, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	storagePath, err := u.cfg.MediaStorage.SaveMedia(ctx, id, input.FileName, input.Data)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	media, err := u.cfg.MediaRepo.CreateMediaAttachment(ctx, nil, repos.CreateMediaAttachmentInput{LocalAccountID: account.ID, FileName: input.FileName, ContentType: input.ContentType, StoragePath: storagePath, Description: input.Description})
	if err != nil {
		_ = u.cfg.MediaStorage.DeleteMedia(ctx, storagePath)
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return media, nil
}

func (u UseCase) GetMedia(ctx context.Context, id string) (*models.MediaAttachment, *domainerrors.DomainError) {
	media, err := u.cfg.MediaRepo.GetMediaAttachmentByID(ctx, nil, id)
	if err != nil {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "media not found")
	}
	if media.StoragePath == "" {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "media not found")
	}
	data, err := u.cfg.MediaStorage.ReadMedia(ctx, media.StoragePath)
	if err != nil {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "media not found")
	}
	media.Data = data
	return media, nil
}

func (u UseCase) UpdateMedia(ctx context.Context, account *models.Account, id string, input UpdateMediaInput) (*models.MediaAttachment, *domainerrors.DomainError) {
	media, derr := u.getOwnedMedia(ctx, account, id)
	if derr != nil {
		return nil, derr
	}
	updated, err := u.cfg.MediaRepo.UpdateMediaAttachmentDescription(ctx, nil, media.ID, input.Description)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return updated, nil
}

func (u UseCase) CleanupMedia(ctx context.Context, olderThan time.Duration, limit int) (*CleanupMediaResult, *domainerrors.DomainError) {
	if olderThan <= 0 {
		olderThan = 24 * time.Hour
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	res := &CleanupMediaResult{}
	broken, err := u.cfg.MediaRepo.ListMediaWithoutStorage(ctx, nil, limit)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	for _, media := range broken {
		if err := u.cfg.MediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		res.DeletedBroken++
	}
	remaining := limit - res.DeletedBroken
	if remaining <= 0 {
		return res, nil
	}
	unattached, err := u.cfg.MediaRepo.ListUnattachedMediaOlderThan(ctx, nil, time.Now().UTC().Add(-olderThan), remaining)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	for _, media := range unattached {
		if err := u.deleteMediaFilesAndRow(ctx, media); err != nil {
			return nil, err
		}
		res.DeletedUnattached++
	}
	return res, nil
}

func (u UseCase) DeleteMedia(ctx context.Context, account *models.Account, id string) *domainerrors.DomainError {
	media, derr := u.getOwnedMedia(ctx, account, id)
	if derr != nil {
		return derr
	}
	attached, err := u.cfg.MediaRepo.MediaAttachmentIsAttached(ctx, nil, media.ID)
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if attached {
		return domainerrors.New(domainerrors.ErrBadRequest, "media is already attached to a status")
	}
	return u.deleteMediaFilesAndRow(ctx, *media)
}

func (u UseCase) deleteMediaFilesAndRow(ctx context.Context, media models.MediaAttachment) *domainerrors.DomainError {
	if err := u.cfg.MediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if err := u.cfg.MediaStorage.DeleteMedia(ctx, media.StoragePath); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return nil
}

func (u UseCase) cleanupUnreferencedMedia(ctx context.Context, media []models.MediaAttachment) *domainerrors.DomainError {
	for _, item := range media {
		attached, err := u.cfg.MediaRepo.MediaAttachmentIsAttached(ctx, nil, item.ID)
		if err != nil {
			return domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		if attached {
			continue
		}
		if derr := u.deleteMediaFilesAndRow(ctx, item); derr != nil {
			return derr
		}
	}
	return nil
}

func (u UseCase) getOwnedMedia(ctx context.Context, account *models.Account, id string) (*models.MediaAttachment, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	media, err := u.cfg.MediaRepo.GetMediaAttachmentByID(ctx, nil, id)
	if err != nil {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "media not found")
	}
	if media.LocalAccountID != account.ID {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "media not found")
	}
	return media, nil
}
