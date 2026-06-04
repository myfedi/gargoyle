package models

import (
	"time"

	domainmodels "github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type StatusEditHistory struct {
	bun.BaseModel `bun:"table:status_edit_history"`

	ID          string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	NoteID      string    `bun:"type:CHAR(26),nullzero,notnull"`
	Content     string    `bun:",nullzero,notnull"`
	PlainText   string    `bun:",nullzero"`
	Visibility  string    `bun:",nullzero,notnull,default:'public'"`
	Sensitive   bool      `bun:",notnull,default:false"`
	SpoilerText string    `bun:",nullzero"`
	CreatedAt   time.Time `bun:"type:timestamptz,nullzero,notnull"`
}

func (e StatusEditHistory) ToModel(mediaIDs []string) domainmodels.NoteEdit {
	return domainmodels.NoteEdit{ID: e.ID, NoteID: e.NoteID, Content: e.Content, PlainText: e.PlainText, Visibility: e.Visibility, Sensitive: e.Sensitive, SpoilerText: e.SpoilerText, CreatedAt: e.CreatedAt, MediaIDs: mediaIDs}
}

type StatusEditHistoryMedia struct {
	bun.BaseModel `bun:"table:status_edit_history_media"`

	EditID   string `bun:"type:CHAR(26),pk,nullzero,notnull"`
	MediaID  string `bun:"type:CHAR(26),pk,nullzero,notnull"`
	Position int    `bun:",notnull,default:0"`
}
