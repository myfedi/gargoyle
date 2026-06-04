package models

import (
	"time"

	domainmodels "github.com/myfedi/gargoyle/domain/models"
)

type Note struct {
	ID             string     `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	LocalAccountID string     `bun:"type:CHAR(26),nullzero,notnull"`
	ActivityID     string     `bun:"type:CHAR(26),nullzero,notnull,unique"`
	URI            string     `bun:",nullzero,notnull,unique"`
	Content        string     `bun:",nullzero,notnull"`
	PlainText      string     `bun:",nullzero"`
	ObjectType     string     `bun:",nullzero,notnull,default:'Note'"`
	Visibility     string     `bun:",nullzero,notnull,default:'public'"`
	PollMultiple   bool       `bun:",notnull,default:false"`
	PollExpiresAt  *time.Time `bun:"type:timestamptz,nullzero"`
	Sensitive      bool       `bun:",notnull,default:false"`
	SpoilerText    string     `bun:",nullzero"`
	AttributedTo   string     `bun:",nullzero,notnull"`
	InReplyToID    *string    `bun:"type:CHAR(26),nullzero"`
	InReplyToURI   *string    `bun:",nullzero"`
	PublishedAt    time.Time  `bun:"type:timestamptz,nullzero,notnull"`
	CreatedAt      time.Time  `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	EditedAt       *time.Time `bun:"type:timestamptz,nullzero"`
}

func (n Note) ToModel() domainmodels.Note {
	return domainmodels.Note{
		ID:             n.ID,
		LocalAccountID: n.LocalAccountID,
		ActivityID:     n.ActivityID,
		URI:            n.URI,
		Content:        n.Content,
		PlainText:      n.PlainText,
		ObjectType:     n.ObjectType,
		Visibility:     n.Visibility,
		PollMultiple:   n.PollMultiple,
		PollExpiresAt:  n.PollExpiresAt,
		Sensitive:      n.Sensitive,
		SpoilerText:    n.SpoilerText,
		AttributedTo:   n.AttributedTo,
		InReplyToID:    n.InReplyToID,
		InReplyToURI:   n.InReplyToURI,
		PublishedAt:    n.PublishedAt,
		CreatedAt:      n.CreatedAt,
		EditedAt:       n.EditedAt,
	}
}
