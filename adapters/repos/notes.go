package repos

import (
	"context"
	"errors"

	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	"github.com/myfedi/gargoyle/domain/models"
	dbPorts "github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	dbUtils "github.com/myfedi/gargoyle/infrastructure/db"
	dbModels "github.com/myfedi/gargoyle/infrastructure/db/models"
	"github.com/uptrace/bun"
)

type NotesRepo struct{ db bun.IDB }

func NewNotesRepo(db *bun.DB) *NotesRepo { return &NotesRepo{db: db} }

var _ repos.NotesRepository = &NotesRepo{}

func (r *NotesRepo) GetLocalPostsCount(ctx context.Context) (int, error) {
	return r.db.NewSelect().Model((*dbModels.Note)(nil)).Count(ctx)
}

func (r *NotesRepo) CreateNote(ctx context.Context, tx *dbPorts.Tx, input repos.CreateNoteInput) (*models.Note, error) {
	db := r.db
	if tx != nil {
		adapted, ok := (*tx).(dbAdapters.BunTx)
		if !ok {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
		db = adapted.Unwrap()
	}

	ulid, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	note := &dbModels.Note{
		ID:             ulid,
		LocalAccountID: input.LocalAccountID,
		ActivityID:     input.ActivityID,
		URI:            input.URI,
		Content:        input.Content,
		PlainText:      input.PlainText,
		AttributedTo:   input.AttributedTo,
		PublishedAt:    input.PublishedAt,
	}
	_, err = db.NewInsert().Model(note).Exec(ctx)
	if err != nil {
		return nil, err
	}
	model := note.ToModel()
	return &model, nil
}

func (r *NotesRepo) UpdateNoteByURI(ctx context.Context, tx *dbPorts.Tx, uri string, content string, plainText string) error {
	db := r.db
	if tx != nil {
		adapted, ok := (*tx).(dbAdapters.BunTx)
		if !ok {
			return errors.New("internal error: unexpected tx implementation provided")
		}
		db = adapted.Unwrap()
	}

	_, err := db.NewUpdate().
		Model((*dbModels.Note)(nil)).
		Set("content = ?", content).
		Set("plain_text = ?", plainText).
		Where("uri = ?", uri).
		Exec(ctx)
	return err
}

func (r *NotesRepo) DeleteNoteByURI(ctx context.Context, tx *dbPorts.Tx, uri string) error {
	db := r.db
	if tx != nil {
		adapted, ok := (*tx).(dbAdapters.BunTx)
		if !ok {
			return errors.New("internal error: unexpected tx implementation provided")
		}
		db = adapted.Unwrap()
	}

	_, err := db.NewDelete().Model((*dbModels.Note)(nil)).Where("uri = ?", uri).Exec(ctx)
	return err
}

func (r *NotesRepo) ListLocalNotes(ctx context.Context, tx *dbPorts.Tx, localAccountID string) ([]models.Note, error) {
	db := r.db
	if tx != nil {
		adapted, ok := (*tx).(dbAdapters.BunTx)
		if !ok {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
		db = adapted.Unwrap()
	}

	var notes []dbModels.Note
	err := db.NewSelect().
		Model(&notes).
		Where("local_account_id = ?", localAccountID).
		Order("published_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]models.Note, 0, len(notes))
	for _, note := range notes {
		res = append(res, note.ToModel())
	}
	return res, nil
}
