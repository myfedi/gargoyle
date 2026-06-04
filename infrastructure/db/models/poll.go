package models

import (
	"time"

	domainmodels "github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type PollOption struct {
	bun.BaseModel `bun:"table:poll_options"`

	ID         string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	NoteID     string    `bun:"type:CHAR(26),nullzero,notnull"`
	Title      string    `bun:",nullzero,notnull"`
	Position   int       `bun:",notnull,default:0"`
	VotesCount int       `bun:",notnull,default:0"`
	CreatedAt  time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
}

func (p PollOption) ToModel() domainmodels.PollOption {
	return domainmodels.PollOption{ID: p.ID, NoteID: p.NoteID, Title: p.Title, Position: p.Position, VotesCount: p.VotesCount, CreatedAt: p.CreatedAt}
}

type PollVote struct {
	bun.BaseModel `bun:"table:poll_votes"`

	ID             string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	PollOptionID   string    `bun:"type:CHAR(26),nullzero,notnull"`
	NoteID         string    `bun:"type:CHAR(26),nullzero,notnull"`
	LocalAccountID *string   `bun:"type:CHAR(26),nullzero"`
	RemoteActor    *string   `bun:",nullzero"`
	CreatedAt      time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
}

func (v PollVote) ToModel() domainmodels.PollVote {
	return domainmodels.PollVote{ID: v.ID, PollOptionID: v.PollOptionID, LocalAccountID: v.LocalAccountID, RemoteActor: v.RemoteActor, CreatedAt: v.CreatedAt}
}
