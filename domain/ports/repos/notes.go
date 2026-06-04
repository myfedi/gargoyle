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
	ObjectType     string
	Visibility     string
	PollMultiple   bool
	PollExpiresAt  *time.Time
	Sensitive      bool
	SpoilerText    string
	AttributedTo   string
	InReplyToID    *string
	InReplyToURI   *string
	PublishedAt    time.Time
}

type UpdateNoteInput struct {
	Content       string
	PlainText     string
	ObjectType    string
	Visibility    string
	PollMultiple  bool
	PollExpiresAt *time.Time
	Sensitive     bool
	SpoilerText   string
}

type CreateNoteEditInput struct {
	Note      models.Note
	CreatedAt time.Time
	MediaIDs  []string
}

type NotesRepository interface {
	PostsRepository
	CreateNote(ctx context.Context, tx *db.Tx, input CreateNoteInput) (*models.Note, error)
	GetNoteByID(ctx context.Context, tx *db.Tx, id string) (*models.Note, error)
	GetNoteByURI(ctx context.Context, tx *db.Tx, uri string) (*models.Note, error)
	UpdateNoteByID(ctx context.Context, tx *db.Tx, id string, input UpdateNoteInput) (*models.Note, error)
	UpdateNoteByURI(ctx context.Context, tx *db.Tx, uri, content, plainText, objectType string) error
	CreateNoteEdit(ctx context.Context, tx *db.Tx, input CreateNoteEditInput) (*models.NoteEdit, error)
	ListNoteEdits(ctx context.Context, tx *db.Tx, noteID string) ([]models.NoteEdit, error)
	DeleteNoteByID(ctx context.Context, tx *db.Tx, id string) error
	DeleteNoteByURI(ctx context.Context, tx *db.Tx, uri string) error
	ListLocalNotes(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Note, error)
	ListLocalNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit int, maxID string) ([]models.Note, error)
	ListHomeTimelineNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, actorURIs []string, limit int, maxID string) ([]models.Note, error)
	ListDirectNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit int, maxID string) ([]models.Note, error)
	ListKnownPublicTimelineNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit int, maxID string) ([]models.Note, error)
	ListKnownLocalTimelineNotesPaged(ctx context.Context, tx *db.Tx, localAccountID, localActorPrefix string, limit int, maxID string) ([]models.Note, error)
	ListKnownRemoteTimelineNotesPaged(ctx context.Context, tx *db.Tx, localAccountID, localActorPrefix string, limit int, maxID string) ([]models.Note, error)
	ListAttributedNotesPaged(ctx context.Context, tx *db.Tx, localAccountID, attributedTo string, limit int, maxID string) ([]models.Note, error)
	ListReplies(ctx context.Context, tx *db.Tx, localAccountID, parentID, parentURI string) ([]models.Note, error)
}
