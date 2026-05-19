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
	AttributedTo   string    `bun:",nullzero,notnull"`
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
		AttributedTo:   n.AttributedTo,
		PublishedAt:    n.PublishedAt,
		CreatedAt:      n.CreatedAt,
	}
}
