package repos

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type CreateNoteInput struct {
	LocalAccountID string
	ActivityID     string
	URI            string
	Content        string
	PlainText      string
	AttributedTo   string
	PublishedAt    time.Time
}

type NotesRepository interface {
	PostsRepository
	CreateNote(ctx context.Context, tx *db.Tx, input CreateNoteInput) (*models.Note, error)
	GetNoteByURI(ctx context.Context, tx *db.Tx, uri string) (*models.Note, error)
	UpdateNoteByURI(ctx context.Context, tx *db.Tx, uri string, content string, plainText string) error
	DeleteNoteByURI(ctx context.Context, tx *db.Tx, uri string) error
	ListLocalNotes(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Note, error)
}
