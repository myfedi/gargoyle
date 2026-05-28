package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type StatusInteraction struct {
	bun.BaseModel  `bun:"table:status_interactions"`
	ID             string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt      time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	LocalAccountID string    `bun:"type:CHAR(26),nullzero,notnull"`
	NoteID         string    `bun:"type:CHAR(26),nullzero,notnull"`
	Type           string    `bun:",nullzero,notnull"`
}

func (s StatusInteraction) ToModel() models.StatusInteraction {
	return models.StatusInteraction{ID: s.ID, CreatedAt: s.CreatedAt, LocalAccountID: s.LocalAccountID, NoteID: s.NoteID, Type: s.Type}
}

type Notification struct {
	bun.BaseModel  `bun:"table:notifications"`
	ID             string     `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt      time.Time  `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	LocalAccountID string     `bun:"type:CHAR(26),nullzero,notnull"`
	ActorAccountID string     `bun:"type:CHAR(26),nullzero,notnull"`
	Type           string     `bun:",nullzero,notnull"`
	StatusID       *string    `bun:"type:CHAR(26),nullzero"`
	ReadAt         *time.Time `bun:"type:timestamptz"`
}

func (n Notification) ToModel() models.Notification {
	return models.Notification{ID: n.ID, CreatedAt: n.CreatedAt, LocalAccountID: n.LocalAccountID, ActorAccountID: n.ActorAccountID, Type: n.Type, StatusID: n.StatusID, ReadAt: n.ReadAt}
}
