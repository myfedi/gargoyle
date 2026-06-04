package repos

import (
	"context"
	"errors"
	"time"

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
var _ repos.CommentsRepository = &NotesRepo{}

func noteVisibility(visibility string) string {
	if visibility == "" {
		return "public"
	}
	return visibility
}

func (r *NotesRepo) GetLocalPostsCount(ctx context.Context) (int, error) {
	return r.db.NewSelect().Model((*dbModels.Note)(nil)).Count(ctx)
}

func (r *NotesRepo) GetLocalCommentsCount(ctx context.Context) (int, error) {
	return r.db.NewSelect().Model((*dbModels.Note)(nil)).Where("in_reply_to_id IS NOT NULL OR in_reply_to_uri IS NOT NULL").Count(ctx)
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
		Visibility:     noteVisibility(input.Visibility),
		Sensitive:      input.Sensitive,
		SpoilerText:    input.SpoilerText,
		AttributedTo:   input.AttributedTo,
		InReplyToID:    input.InReplyToID,
		InReplyToURI:   input.InReplyToURI,
		PublishedAt:    input.PublishedAt,
	}
	_, err = db.NewInsert().Model(note).Exec(ctx)
	if err != nil {
		return nil, err
	}
	model := note.ToModel()
	return &model, nil
}

func (r *NotesRepo) GetNoteByID(ctx context.Context, tx *dbPorts.Tx, id string) (*models.Note, error) {
	db := r.db
	if tx != nil {
		adapted, ok := (*tx).(dbAdapters.BunTx)
		if !ok {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
		db = adapted.Unwrap()
	}

	var note dbModels.Note
	err := db.NewSelect().Model(&note).Where("id = ?", id).Limit(1).Scan(ctx)
	if err != nil {
		return nil, err
	}
	model := note.ToModel()
	return &model, nil
}

func (r *NotesRepo) GetNoteByURI(ctx context.Context, tx *dbPorts.Tx, uri string) (*models.Note, error) {
	db := r.db
	if tx != nil {
		adapted, ok := (*tx).(dbAdapters.BunTx)
		if !ok {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
		db = adapted.Unwrap()
	}

	var note dbModels.Note
	err := db.NewSelect().Model(&note).Where("uri = ?", uri).Limit(1).Scan(ctx)
	if err != nil {
		return nil, err
	}
	model := note.ToModel()
	return &model, nil
}

func (r *NotesRepo) UpdateNoteByID(ctx context.Context, tx *dbPorts.Tx, id string, input repos.UpdateNoteInput) (*models.Note, error) {
	db := r.db
	if tx != nil {
		adapted, ok := (*tx).(dbAdapters.BunTx)
		if !ok {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
		db = adapted.Unwrap()
	}

	now := time.Now().UTC()
	_, err := db.NewUpdate().
		Model((*dbModels.Note)(nil)).
		Set("content = ?", input.Content).
		Set("plain_text = ?", input.PlainText).
		Set("visibility = ?", noteVisibility(input.Visibility)).
		Set("sensitive = ?", input.Sensitive).
		Set("spoiler_text = ?", input.SpoilerText).
		Set("edited_at = ?", now).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetNoteByID(ctx, tx, id)
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

	var existing dbModels.Note
	if err := db.NewSelect().Model(&existing).Where("uri = ?", uri).Limit(1).Scan(ctx); err != nil {
		return err
	}
	createdAt := existing.PublishedAt
	if existing.EditedAt != nil {
		createdAt = *existing.EditedAt
	}
	ulid, err := dbUtils.NewULID()
	if err != nil {
		return err
	}
	if _, err := db.NewInsert().Model(&dbModels.StatusEditHistory{ID: ulid, NoteID: existing.ID, Content: existing.Content, PlainText: existing.PlainText, Visibility: noteVisibility(existing.Visibility), Sensitive: existing.Sensitive, SpoilerText: existing.SpoilerText, CreatedAt: createdAt}).Exec(ctx); err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = db.NewUpdate().
		Model((*dbModels.Note)(nil)).
		Set("content = ?", content).
		Set("plain_text = ?", plainText).
		Set("edited_at = ?", now).
		Where("uri = ?", uri).
		Exec(ctx)
	return err
}

func (r *NotesRepo) CreateNoteEdit(ctx context.Context, tx *dbPorts.Tx, input repos.CreateNoteEditInput) (*models.NoteEdit, error) {
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
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = input.Note.PublishedAt
	}
	if createdAt.IsZero() {
		createdAt = input.Note.CreatedAt
	}
	edit := &dbModels.StatusEditHistory{ID: ulid, NoteID: input.Note.ID, Content: input.Note.Content, PlainText: input.Note.PlainText, Visibility: noteVisibility(input.Note.Visibility), Sensitive: input.Note.Sensitive, SpoilerText: input.Note.SpoilerText, CreatedAt: createdAt}
	if _, err := db.NewInsert().Model(edit).Exec(ctx); err != nil {
		return nil, err
	}
	for i, mediaID := range input.MediaIDs {
		if _, err := db.NewInsert().Model(&dbModels.StatusEditHistoryMedia{EditID: ulid, MediaID: mediaID, Position: i}).Exec(ctx); err != nil {
			return nil, err
		}
	}
	model := edit.ToModel(input.MediaIDs)
	return &model, nil
}

func (r *NotesRepo) ListNoteEdits(ctx context.Context, tx *dbPorts.Tx, noteID string) ([]models.NoteEdit, error) {
	db := r.db
	if tx != nil {
		adapted, ok := (*tx).(dbAdapters.BunTx)
		if !ok {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
		db = adapted.Unwrap()
	}

	var rows []dbModels.StatusEditHistory
	if err := db.NewSelect().Model(&rows).Where("note_id = ?", noteID).Order("created_at ASC", "id ASC").Scan(ctx); err != nil {
		return nil, err
	}
	res := make([]models.NoteEdit, 0, len(rows))
	for _, row := range rows {
		var mediaRows []dbModels.StatusEditHistoryMedia
		if err := db.NewSelect().Model(&mediaRows).Where("edit_id = ?", row.ID).Order("position ASC").Scan(ctx); err != nil {
			return nil, err
		}
		mediaIDs := make([]string, 0, len(mediaRows))
		for _, media := range mediaRows {
			mediaIDs = append(mediaIDs, media.MediaID)
		}
		res = append(res, row.ToModel(mediaIDs))
	}
	return res, nil
}

func (r *NotesRepo) DeleteNoteByID(ctx context.Context, tx *dbPorts.Tx, id string) error {
	db := r.db
	if tx != nil {
		adapted, ok := (*tx).(dbAdapters.BunTx)
		if !ok {
			return errors.New("internal error: unexpected tx implementation provided")
		}
		db = adapted.Unwrap()
	}

	_, err := db.NewDelete().Model((*dbModels.Note)(nil)).Where("id = ?", id).Exec(ctx)
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
	return r.ListLocalNotesPaged(ctx, tx, localAccountID, 0, "")
}

func (r *NotesRepo) ListLocalNotesPaged(ctx context.Context, tx *dbPorts.Tx, localAccountID string, limit int, maxID string) ([]models.Note, error) {
	return r.listNotes(ctx, tx, noteListFilter{localAccountID: localAccountID, limit: limit, maxID: maxID})
}

func (r *NotesRepo) ListDirectNotesPaged(ctx context.Context, tx *dbPorts.Tx, localAccountID string, limit int, maxID string) ([]models.Note, error) {
	return r.listNotes(ctx, tx, noteListFilter{localAccountID: localAccountID, visibility: "direct", limit: limit, maxID: maxID})
}

func (r *NotesRepo) ListKnownPublicTimelineNotesPaged(ctx context.Context, tx *dbPorts.Tx, localAccountID string, limit int, maxID string) ([]models.Note, error) {
	return r.listNotes(ctx, tx, noteListFilter{localAccountID: localAccountID, publicOnly: true, limit: limit, maxID: maxID})
}

func (r *NotesRepo) ListKnownLocalTimelineNotesPaged(ctx context.Context, tx *dbPorts.Tx, localAccountID string, localActorPrefix string, limit int, maxID string) ([]models.Note, error) {
	return r.listNotes(ctx, tx, noteListFilter{localAccountID: localAccountID, localActorPrefix: localActorPrefix, localOnly: true, publicOnly: true, limit: limit, maxID: maxID})
}

func (r *NotesRepo) ListKnownRemoteTimelineNotesPaged(ctx context.Context, tx *dbPorts.Tx, localAccountID string, localActorPrefix string, limit int, maxID string) ([]models.Note, error) {
	return r.listNotes(ctx, tx, noteListFilter{localAccountID: localAccountID, localActorPrefix: localActorPrefix, remoteOnly: true, publicOnly: true, limit: limit, maxID: maxID})
}

func (r *NotesRepo) ListAttributedNotesPaged(ctx context.Context, tx *dbPorts.Tx, localAccountID string, attributedTo string, limit int, maxID string) ([]models.Note, error) {
	return r.listNotes(ctx, tx, noteListFilter{localAccountID: localAccountID, attributedTo: attributedTo, limit: limit, maxID: maxID})
}

type noteListFilter struct {
	localAccountID   string
	attributedTo     string
	localActorPrefix string
	localOnly        bool
	remoteOnly       bool
	publicOnly       bool
	visibility       string
	limit            int
	maxID            string
}

func (r *NotesRepo) ListReplies(ctx context.Context, tx *dbPorts.Tx, localAccountID string, parentID string, parentURI string) ([]models.Note, error) {
	db := r.db
	if tx != nil {
		adapted, ok := (*tx).(dbAdapters.BunTx)
		if !ok {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
		db = adapted.Unwrap()
	}
	var notes []dbModels.Note
	query := db.NewSelect().Model(&notes).Where("local_account_id = ?", localAccountID)
	if parentURI != "" {
		query = query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("in_reply_to_id = ?", parentID).WhereOr("in_reply_to_uri = ?", parentURI)
		})
	} else {
		query = query.Where("in_reply_to_id = ?", parentID)
	}
	err := query.Order("published_at ASC", "id ASC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]models.Note, 0, len(notes))
	for _, note := range notes {
		res = append(res, note.ToModel())
	}
	return res, nil
}

func (r *NotesRepo) listNotes(ctx context.Context, tx *dbPorts.Tx, filter noteListFilter) ([]models.Note, error) {
	db := r.db
	if tx != nil {
		adapted, ok := (*tx).(dbAdapters.BunTx)
		if !ok {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
		db = adapted.Unwrap()
	}

	var notes []dbModels.Note
	query := db.NewSelect().Model(&notes).Where("local_account_id = ?", filter.localAccountID).Order("published_at DESC", "id DESC")
	if filter.attributedTo != "" {
		query = query.Where("attributed_to = ?", filter.attributedTo)
	}
	if filter.localOnly {
		query = query.Where("attributed_to LIKE ?", filter.localActorPrefix+"%")
	}
	if filter.remoteOnly {
		query = query.Where("attributed_to NOT LIKE ?", filter.localActorPrefix+"%")
	}
	if filter.publicOnly {
		query = query.Where("visibility = ?", "public")
	}
	if filter.visibility != "" {
		query = query.Where("visibility = ?", filter.visibility)
	}
	if filter.maxID != "" {
		query = query.Where("id < ?", filter.maxID)
	}
	if filter.limit > 0 {
		query = query.Limit(filter.limit)
	}
	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]models.Note, 0, len(notes))
	for _, note := range notes {
		res = append(res, note.ToModel())
	}
	return res, nil
}
