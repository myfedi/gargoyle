package clientapi

import (
	"context"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

func remoteProfileImagesCached(account *models.Account) bool {
	if account == nil {
		return true
	}
	avatarCached := account.AvatarURL == nil || *account.AvatarURL == "" || account.AvatarMediaID != nil
	headerCached := account.HeaderURL == nil || *account.HeaderURL == "" || account.HeaderMediaID != nil
	return avatarCached && headerCached
}

const remoteProfileImageCacheTimeout = 8 * time.Second

var (
	remoteProfileImageCacheSlots    = make(chan struct{}, 2)
	remoteProfileImageCacheInflight sync.Map
)

func cacheRemoteAccountProfileImagesAsync(mediaRepo repos.MediaRepository, mediaStorage ports.MediaStorage, remoteMediaFetcher ports.RemoteMediaFetcher, remoteAccountsRepo repos.RemoteAccountsRepository, notifier RemoteProfileCacheNotifier, localAccountID string, remote *models.Account) {
	if remote == nil || remote.URI == "" || localAccountID == "" || remoteAccountsRepo == nil {
		return
	}
	remoteCopy := *remote
	clearMissingRemoteProfileImageMedia(mediaRepo, &remoteCopy)
	if remoteProfileImagesCached(&remoteCopy) {
		return
	}
	key := localAccountID + "\x00" + remote.URI
	if _, loaded := remoteProfileImageCacheInflight.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	select {
	case remoteProfileImageCacheSlots <- struct{}{}:
	default:
		remoteProfileImageCacheInflight.Delete(key)
		return
	}
	go func() {
		defer func() {
			<-remoteProfileImageCacheSlots
			remoteProfileImageCacheInflight.Delete(key)
		}()
		ctx, cancel := context.WithTimeout(context.Background(), remoteProfileImageCacheTimeout)
		defer cancel()
		cacheRemoteAccountProfileImages(ctx, nil, mediaRepo, mediaStorage, remoteMediaFetcher, localAccountID, &remoteCopy)
		if remoteCopy.AvatarMediaID == remote.AvatarMediaID && remoteCopy.HeaderMediaID == remote.HeaderMediaID {
			return
		}
		if _, err := remoteAccountsRepo.UpsertRemoteAccount(ctx, nil, remoteCopy); err == nil && notifier != nil {
			notifier.RemoteProfileImagesCached(localAccountID, remoteCopy.URI)
		}
	}()
}

func clearMissingRemoteProfileImageMedia(mediaRepo repos.MediaRepository, remote *models.Account) {
	if mediaRepo == nil || remote == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if remote.AvatarMediaID != nil && *remote.AvatarMediaID != "" {
		if _, err := mediaRepo.GetMediaAttachmentByID(ctx, nil, *remote.AvatarMediaID); err != nil {
			remote.AvatarMediaID = nil
		}
	}
	if remote.HeaderMediaID != nil && *remote.HeaderMediaID != "" {
		if _, err := mediaRepo.GetMediaAttachmentByID(ctx, nil, *remote.HeaderMediaID); err != nil {
			remote.HeaderMediaID = nil
		}
	}
}

func cacheRemoteAccountProfileImages(ctx context.Context, tx *db.Tx, mediaRepo repos.MediaRepository, mediaStorage ports.MediaStorage, remoteMediaFetcher ports.RemoteMediaFetcher, localAccountID string, remote *models.Account) {
	if remote == nil || localAccountID == "" {
		return
	}
	if remote.AvatarMediaID == nil {
		remote.AvatarMediaID = cacheRemoteProfileImage(ctx, tx, mediaRepo, mediaStorage, remoteMediaFetcher, localAccountID, remote.AvatarURL)
	}
	if remote.HeaderMediaID == nil {
		remote.HeaderMediaID = cacheRemoteProfileImage(ctx, tx, mediaRepo, mediaStorage, remoteMediaFetcher, localAccountID, remote.HeaderURL)
	}
}

func cacheRemoteProfileImage(ctx context.Context, tx *db.Tx, mediaRepo repos.MediaRepository, mediaStorage ports.MediaStorage, remoteMediaFetcher ports.RemoteMediaFetcher, localAccountID string, rawURL *string) *string {
	if mediaRepo == nil || mediaStorage == nil || remoteMediaFetcher == nil || rawURL == nil || *rawURL == "" {
		return nil
	}
	if media, err := mediaRepo.GetMediaAttachmentByRemoteURL(ctx, tx, *rawURL); err == nil {
		_ = mediaRepo.MarkMediaAccessed(ctx, tx, media.ID, time.Now().UTC())
		return &media.ID
	}
	fetched, err := remoteMediaFetcher.FetchMedia(ctx, *rawURL, MaxMediaUploadBytes)
	if err != nil || !strings.HasPrefix(fetched.ContentType, "image/") {
		return nil
	}
	fileName := fetched.FileName
	if fileName == "" {
		fileName = path.Base(remoteMediaCacheKey(*rawURL))
	}
	now := time.Now().UTC()
	storagePath, err := mediaStorage.SaveMedia(ctx, remoteMediaCacheKey(*rawURL), fileName, fetched.Data)
	if err != nil {
		return nil
	}
	media, err := mediaRepo.CreateMediaAttachment(ctx, tx, repos.CreateMediaAttachmentInput{LocalAccountID: localAccountID, FileName: fileName, ContentType: fetched.ContentType, StoragePath: storagePath, RemoteURL: rawURL, RemoteFetchedAt: &now, RemoteLastAccessedAt: &now, FileSize: int64(len(fetched.Data))})
	if err != nil {
		_ = mediaStorage.DeleteMedia(ctx, storagePath)
		return nil
	}
	return &media.ID
}
