package repos

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type CreateMediaAttachmentInput struct {
	LocalAccountID string
	FileName       string
	ContentType    string
	Data           []byte
	Description    string
}

type MediaRepository interface {
	CreateMediaAttachment(ctx context.Context, tx *db.Tx, input CreateMediaAttachmentInput) (*models.MediaAttachment, error)
	GetMediaAttachmentByID(ctx context.Context, tx *db.Tx, id string) (*models.MediaAttachment, error)
	UpdateMediaAttachmentDescription(ctx context.Context, tx *db.Tx, id string, description string) (*models.MediaAttachment, error)
	DeleteMediaAttachment(ctx context.Context, tx *db.Tx, id string) error
	MediaAttachmentIsAttached(ctx context.Context, tx *db.Tx, id string) (bool, error)
	AttachMediaToNote(ctx context.Context, tx *db.Tx, noteID string, mediaID string) error
	ListMediaForNote(ctx context.Context, tx *db.Tx, noteID string) ([]models.MediaAttachment, error)
}
