package models

import (
	"time"

	"github.com/uptrace/bun"
)

type ConversationDismissal struct {
	bun.BaseModel `bun:"table:conversation_dismissals"`

	LocalAccountID string    `bun:"type:CHAR(26),pk,nullzero,notnull"`
	ConversationID string    `bun:",pk,nullzero,notnull"`
	CreatedAt      time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
}
