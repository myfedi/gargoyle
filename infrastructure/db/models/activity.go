package models

import (
	"time"

	domainmodels "github.com/myfedi/gargoyle/domain/models"
)

type Activity struct {
	ID             string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	LocalAccountID string    `bun:"type:CHAR(26),nullzero,notnull"`
	Direction      string    `bun:",nullzero,notnull"`
	Type           string    `bun:",nullzero,notnull"`
	Actor          string    `bun:",nullzero,notnull"`
	Object         string    `bun:",nullzero"`
	RawJSON        string    `bun:",nullzero,notnull"`
	CreatedAt      time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
}

func (a Activity) ToModel() domainmodels.Activity {
	return domainmodels.Activity{
		ID:             a.ID,
		LocalAccountID: a.LocalAccountID,
		Direction:      domainmodels.ActivityDirection(a.Direction),
		Type:           a.Type,
		Actor:          a.Actor,
		Object:         a.Object,
		RawJSON:        a.RawJSON,
		CreatedAt:      a.CreatedAt,
	}
}

type Follow struct {
	ID             string     `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	LocalAccountID string     `bun:"type:CHAR(26),nullzero,notnull"`
	RemoteActor    string     `bun:",nullzero,notnull,unique:follows_local_actor_uniq"`
	RemoteInbox    *string    `bun:",nullzero"`
	ActivityID     string     `bun:"type:CHAR(26),nullzero,notnull"`
	Direction      string     `bun:",nullzero,notnull"`
	AcceptedAt     *time.Time `bun:"type:timestamptz,nullzero"`
	CreatedAt      time.Time  `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
}

func (f Follow) ToModel() domainmodels.Follow {
	return domainmodels.Follow{
		ID:             f.ID,
		LocalAccountID: f.LocalAccountID,
		RemoteActor:    f.RemoteActor,
		RemoteInbox:    f.RemoteInbox,
		ActivityID:     f.ActivityID,
		Direction:      f.Direction,
		AcceptedAt:     f.AcceptedAt,
		CreatedAt:      f.CreatedAt,
	}
}
