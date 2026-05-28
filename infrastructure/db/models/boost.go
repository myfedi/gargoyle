package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type Boost struct {
	bun.BaseModel `bun:"table:boosts"`

	ID             string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt      time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	LocalAccountID string    `bun:"type:CHAR(26),nullzero,notnull"`
	Actor          string    `bun:",nullzero,notnull"`
	NoteID         string    `bun:"type:CHAR(26),nullzero,notnull"`
	URI            string    `bun:",nullzero,notnull,unique"`
	PublishedAt    time.Time `bun:"type:timestamptz,nullzero,notnull"`
}

func (b Boost) ToModel() models.Boost {
	return models.Boost{ID: b.ID, CreatedAt: b.CreatedAt, LocalAccountID: b.LocalAccountID, Actor: b.Actor, NoteID: b.NoteID, URI: b.URI, PublishedAt: b.PublishedAt}
}
