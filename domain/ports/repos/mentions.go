package repos

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type CreateMentionInput struct {
	LocalAccountID string
	NoteID         string
	AccountID      string
	Username       string
	Acct           string
	URL            string
}

type MentionsRepository interface {
	CreateMention(ctx context.Context, tx *db.Tx, input CreateMentionInput) (*models.Mention, error)
	DeleteMentionsForNote(ctx context.Context, tx *db.Tx, noteID string) error
	ListMentionsForNote(ctx context.Context, tx *db.Tx, noteID string) ([]models.Mention, error)
}
