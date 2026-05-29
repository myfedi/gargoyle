package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type Mention struct {
	bun.BaseModel `bun:"table:mentions"`

	ID             string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt      time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	LocalAccountID string    `bun:"type:CHAR(26),nullzero,notnull"`
	NoteID         string    `bun:"type:CHAR(26),nullzero,notnull"`
	AccountID      string    `bun:",nullzero,notnull"`
	Username       string    `bun:",nullzero,notnull"`
	Acct           string    `bun:",nullzero,notnull"`
	URL            string    `bun:",nullzero,notnull"`
}

func (m Mention) ToModel() models.Mention {
	return models.Mention{ID: m.ID, CreatedAt: m.CreatedAt, LocalAccountID: m.LocalAccountID, NoteID: m.NoteID, AccountID: m.AccountID, Username: m.Username, Acct: m.Acct, URL: m.URL}
}
