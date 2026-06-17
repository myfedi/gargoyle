package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type RelaySubscription struct {
	bun.BaseModel `bun:"table:relay_subscriptions"`

	ID              string     `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt       time.Time  `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time  `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	ActorURI        string     `bun:",nullzero,notnull,unique"`
	InboxURI        string     `bun:",nullzero,notnull"`
	Status          string     `bun:",nullzero,notnull,default:'pending'"`
	AcceptedAt      *time.Time `bun:"type:timestamptz"`
	CreatedByUserID string     `bun:"type:CHAR(26),nullzero,notnull"`
	LastError       *string
}

func (r RelaySubscription) ToModel() models.RelaySubscription {
	return models.RelaySubscription{ID: r.ID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ActorURI: r.ActorURI, InboxURI: r.InboxURI, Status: r.Status, AcceptedAt: r.AcceptedAt, CreatedByUserID: r.CreatedByUserID, LastError: r.LastError}
}
