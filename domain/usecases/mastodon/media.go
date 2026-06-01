package mastodon

import (
	"context"

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
	if err := u.cfg.MediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if err := u.cfg.MediaStorage.DeleteMedia(ctx, media.StoragePath); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
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
