package models

import (
	"time"

	domainmodels "github.com/myfedi/gargoyle/domain/models"
)

type Note struct {
	ID             string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	LocalAccountID string    `bun:"type:CHAR(26),nullzero,notnull"`
	ActivityID     string    `bun:"type:CHAR(26),nullzero,notnull,unique"`
	URI            string    `bun:",nullzero,notnull,unique"`
	Content        string    `bun:",nullzero,notnull"`
	PlainText      string    `bun:",nullzero"`
	Visibility     string    `bun:",nullzero,notnull,default:'public'"`
	Sensitive      bool      `bun:",notnull,default:false"`
	SpoilerText    string    `bun:",nullzero"`
	AttributedTo   string    `bun:",nullzero,notnull"`
	InReplyToID    *string   `bun:"type:CHAR(26),nullzero"`
	InReplyToURI   *string   `bun:",nullzero"`
	PublishedAt    time.Time `bun:"type:timestamptz,nullzero,notnull"`
	CreatedAt      time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
}

func (n Note) ToModel() domainmodels.Note {
	return domainmodels.Note{
		ID:             n.ID,
		LocalAccountID: n.LocalAccountID,
		ActivityID:     n.ActivityID,
		URI:            n.URI,
		Content:        n.Content,
		PlainText:      n.PlainText,
		Visibility:     n.Visibility,
		Sensitive:      n.Sensitive,
		SpoilerText:    n.SpoilerText,
		AttributedTo:   n.AttributedTo,
		InReplyToID:    n.InReplyToID,
		InReplyToURI:   n.InReplyToURI,
		PublishedAt:    n.PublishedAt,
		CreatedAt:      n.CreatedAt,
	}
}
