package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type MediaAttachment struct {
	bun.BaseModel `bun:"table:media_attachments"`

	ID             string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt      time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	LocalAccountID string    `bun:"type:CHAR(26),nullzero,notnull"`
	FileName       string
	ContentType    string `bun:",nullzero,notnull"`
	StoragePath    string
	Description    string
}

func (m MediaAttachment) ToModel() models.MediaAttachment {
	return models.MediaAttachment{ID: m.ID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, LocalAccountID: m.LocalAccountID, FileName: m.FileName, ContentType: m.ContentType, StoragePath: m.StoragePath, Description: m.Description}
}
