package clientapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
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

const MaxMediaUploadBytes = 10 << 20

type UpdateMediaInput struct {
	Description string
}

type CleanupMediaResult struct {
	DeletedUnattached int
	DeletedBroken     int
}

func (u Media) UploadMedia(ctx context.Context, account *models.Account, input UploadMediaInput) (*models.MediaAttachment, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	if len(input.Data) == 0 {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "media file is required")
	}
	if len(input.Data) > MaxMediaUploadBytes {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "media file is too large")
	}
	contentType, derr := safeMediaContentType(input.ContentType, input.Data)
	if derr != nil {
		return nil, derr
	}
	id, err := u.deps.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	storagePath, err := u.deps.MediaStorage.SaveMedia(ctx, id, input.FileName, input.Data)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	media, err := u.deps.MediaRepo.CreateMediaAttachment(ctx, nil, repos.CreateMediaAttachmentInput{LocalAccountID: account.ID, FileName: input.FileName, ContentType: contentType, StoragePath: storagePath, FileSize: int64(len(input.Data)), Description: input.Description})
	if err != nil {
		_ = u.deps.MediaStorage.DeleteMedia(ctx, storagePath)
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return media, nil
}

func safeMediaContentType(declared string, data []byte) (string, *domainerrors.DomainError) {
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	sniffed := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	allowed := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
		"video/mp4":  true,
		"audio/mpeg": true,
		"audio/ogg":  true,
		"audio/wav":  true,
	}
	if allowed[sniffed] {
		return sniffed, nil
	}
	if allowed[declared] && strings.HasPrefix(declared, "video/") {
		return declared, nil
	}
	return "", domainerrors.New(domainerrors.ErrBadRequest, "unsupported media content type")
}

func (u Media) GetMedia(ctx context.Context, id string) (*models.MediaAttachment, *domainerrors.DomainError) {
	media, err := u.deps.MediaRepo.GetMediaAttachmentByID(ctx, nil, id)
	if err != nil {
		return nil, domainerrors.New(domainerrors.ErrNotFound, mediaNotFoundMessage)
	}
	if media.StoragePath == "" {
		if derr := u.recacheRemoteMedia(ctx, media); derr != nil {
			return nil, derr
		}
	}
	data, err := u.deps.MediaStorage.ReadMedia(ctx, media.StoragePath)
	if err != nil {
		return nil, domainerrors.New(domainerrors.ErrNotFound, mediaNotFoundMessage)
	}
	if media.RemoteURL != nil && *media.RemoteURL != "" {
		_ = u.deps.MediaRepo.MarkMediaAccessed(ctx, nil, media.ID, time.Now().UTC())
	}
	media.Data = data
	return media, nil
}

func (u Media) recacheRemoteMedia(ctx context.Context, media *models.MediaAttachment) *domainerrors.DomainError {
	if media.RemoteURL == nil || *media.RemoteURL == "" || u.deps.RemoteMediaFetcher == nil {
		return domainerrors.New(domainerrors.ErrNotFound, mediaNotFoundMessage)
	}
	fetched, err := u.deps.RemoteMediaFetcher.FetchMedia(ctx, *media.RemoteURL, MaxMediaUploadBytes)
	if err != nil {
		return domainerrors.New(domainerrors.ErrNotFound, mediaNotFoundMessage)
	}
	contentType, derr := safeMediaContentType(fetched.ContentType, fetched.Data)
	if derr != nil {
		return domainerrors.New(domainerrors.ErrNotFound, mediaNotFoundMessage)
	}
	fileName := strings.TrimSpace(fetched.FileName)
	if fileName == "" || fileName == "." || fileName == "/" {
		fileName = media.FileName
	}
	storagePath, err := u.deps.MediaStorage.SaveMedia(ctx, remoteMediaCacheKey(*media.RemoteURL), fileName, fetched.Data)
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	now := time.Now().UTC()
	if err := u.deps.MediaRepo.UpdateMediaStorage(ctx, nil, media.ID, storagePath, contentType, fileName, int64(len(fetched.Data)), now); err != nil {
		_ = u.deps.MediaStorage.DeleteMedia(ctx, storagePath)
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	media.StoragePath = storagePath
	media.ContentType = contentType
	media.FileName = fileName
	media.FileSize = int64(len(fetched.Data))
	media.RemoteFetchedAt = &now
	media.RemoteLastAccessedAt = &now
	return nil
}

func remoteMediaCacheKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "remote-" + hex.EncodeToString(sum[:])
}

func (u Media) UpdateMedia(ctx context.Context, account *models.Account, id string, input UpdateMediaInput) (*models.MediaAttachment, *domainerrors.DomainError) {
	media, derr := u.getOwnedMedia(ctx, account, id)
	if derr != nil {
		return nil, derr
	}
	updated, err := u.deps.MediaRepo.UpdateMediaAttachmentDescription(ctx, nil, media.ID, input.Description)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return updated, nil
}

func (u Media) CleanupMedia(ctx context.Context, olderThan time.Duration, limit int) (*CleanupMediaResult, *domainerrors.DomainError) {
	if olderThan <= 0 {
		olderThan = 24 * time.Hour
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	res := &CleanupMediaResult{}
	broken, err := u.deps.MediaRepo.ListMediaWithoutStorage(ctx, nil, limit)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	for _, media := range broken {
		if err := u.deps.MediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		res.DeletedBroken++
	}
	remaining := limit - res.DeletedBroken
	if remaining <= 0 {
		return res, nil
	}
	unattached, err := u.deps.MediaRepo.ListUnattachedMediaOlderThan(ctx, nil, time.Now().UTC().Add(-olderThan), remaining)
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

func (u Media) DeleteMedia(ctx context.Context, account *models.Account, id string) *domainerrors.DomainError {
	media, derr := u.getOwnedMedia(ctx, account, id)
	if derr != nil {
		return derr
	}
	attached, err := u.deps.MediaRepo.MediaAttachmentIsAttached(ctx, nil, media.ID)
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if attached {
		return domainerrors.New(domainerrors.ErrBadRequest, "media is already attached to a status")
	}
	return u.deleteMediaFilesAndRow(ctx, *media)
}

func (u Media) deleteMediaFilesAndRow(ctx context.Context, media models.MediaAttachment) *domainerrors.DomainError {
	if err := u.deps.MediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if err := u.deps.MediaStorage.DeleteMedia(ctx, media.StoragePath); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return nil
}

func (u Media) cleanupUnreferencedMedia(ctx context.Context, media []models.MediaAttachment) *domainerrors.DomainError {
	for _, item := range media {
		attached, err := u.deps.MediaRepo.MediaAttachmentIsAttached(ctx, nil, item.ID)
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

func (u Media) getOwnedMedia(ctx context.Context, account *models.Account, id string) (*models.MediaAttachment, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	media, err := u.deps.MediaRepo.GetMediaAttachmentByID(ctx, nil, id)
	if err != nil {
		return nil, domainerrors.New(domainerrors.ErrNotFound, mediaNotFoundMessage)
	}
	if media.LocalAccountID != account.ID {
		return nil, domainerrors.New(domainerrors.ErrNotFound, mediaNotFoundMessage)
	}
	return media, nil
}
