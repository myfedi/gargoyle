package repos

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type CreateMediaAttachmentInput struct {
	LocalAccountID       string
	FileName             string
	ContentType          string
	StoragePath          string
	RemoteURL            *string
	RemoteFetchedAt      *time.Time
	RemoteLastAccessedAt *time.Time
	FileSize             int64
	Description          string
}

type MediaRepository interface {
	CreateMediaAttachment(ctx context.Context, tx *db.Tx, input CreateMediaAttachmentInput) (*models.MediaAttachment, error)
	GetMediaAttachmentByID(ctx context.Context, tx *db.Tx, id string) (*models.MediaAttachment, error)
	GetMediaAttachmentByRemoteURL(ctx context.Context, tx *db.Tx, remoteURL string) (*models.MediaAttachment, error)
	UpdateMediaAttachmentDescription(ctx context.Context, tx *db.Tx, id, description string) (*models.MediaAttachment, error)
	MarkMediaAccessed(ctx context.Context, tx *db.Tx, id string, accessedAt time.Time) error
	UpdateMediaStorage(ctx context.Context, tx *db.Tx, id, storagePath, contentType, fileName string, fileSize int64, fetchedAt time.Time) error
	ClearMediaStorage(ctx context.Context, tx *db.Tx, id string) error
	DeleteMediaAttachment(ctx context.Context, tx *db.Tx, id string) error
	MediaAttachmentIsAttached(ctx context.Context, tx *db.Tx, id string) (bool, error)
	AttachMediaToNote(ctx context.Context, tx *db.Tx, noteID, mediaID string) error
	ReplaceMediaForNote(ctx context.Context, tx *db.Tx, noteID string, mediaIDs []string) error
	ListMediaForNote(ctx context.Context, tx *db.Tx, noteID string) ([]models.MediaAttachment, error)
	ListUnattachedMediaOlderThan(ctx context.Context, tx *db.Tx, cutoff time.Time, limit int) ([]models.MediaAttachment, error)
	ListMediaWithoutStorage(ctx context.Context, tx *db.Tx, limit int) ([]models.MediaAttachment, error)
	ListRemoteCachedMediaOlderThan(ctx context.Context, tx *db.Tx, cutoff time.Time, limit int) ([]models.MediaAttachment, error)
	ListRemoteCachedMediaByLastAccess(ctx context.Context, tx *db.Tx, limit int) ([]models.MediaAttachment, error)
	RemoteCachedMediaSize(ctx context.Context, tx *db.Tx) (int64, error)
}
