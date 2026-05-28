package models

import (
	"time"

	"github.com/uptrace/bun"
)

type NoteMediaAttachment struct {
	bun.BaseModel `bun:"table:note_media_attachments"`

	NoteID    string    `bun:"type:CHAR(26),pk,nullzero,notnull"`
	MediaID   string    `bun:"type:CHAR(26),pk,nullzero,notnull"`
	CreatedAt time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
}
