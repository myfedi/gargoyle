package repos

import (
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
	CreateNote(tx *db.Tx, input CreateNoteInput) (*models.Note, error)
	ListLocalNotes(tx *db.Tx, localAccountID string) ([]models.Note, error)
}
