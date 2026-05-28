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

type MediaRepo struct{ db bun.IDB }

func NewMediaRepo(db *bun.DB) *MediaRepo { return &MediaRepo{db: db} }

var _ repos.MediaRepository = &MediaRepo{}

func (r *MediaRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New("internal error: unexpected tx implementation provided")
	}
	return adapted.Unwrap(), nil
}

func (r *MediaRepo) CreateMediaAttachment(ctx context.Context, tx *dbPorts.Tx, input repos.CreateMediaAttachmentInput) (*models.MediaAttachment, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	row := &dbModels.MediaAttachment{ID: id, LocalAccountID: input.LocalAccountID, FileName: input.FileName, ContentType: input.ContentType, StoragePath: input.StoragePath, Description: input.Description}
	if _, err := db.NewInsert().Model(row).Exec(ctx); err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}

func (r *MediaRepo) GetMediaAttachmentByID(ctx context.Context, tx *dbPorts.Tx, id string) (*models.MediaAttachment, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var row dbModels.MediaAttachment
	if err := db.NewSelect().Model(&row).Where("id = ?", id).Scan(ctx); err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}

func (r *MediaRepo) UpdateMediaAttachmentDescription(ctx context.Context, tx *dbPorts.Tx, id string, description string) (*models.MediaAttachment, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	_, err = db.NewUpdate().Model((*dbModels.MediaAttachment)(nil)).Set("description = ?", description).Set("updated_at = ?", time.Now().UTC()).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetMediaAttachmentByID(ctx, tx, id)
}

func (r *MediaRepo) DeleteMediaAttachment(ctx context.Context, tx *dbPorts.Tx, id string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.MediaAttachment)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (r *MediaRepo) MediaAttachmentIsAttached(ctx context.Context, tx *dbPorts.Tx, id string) (bool, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return false, err
	}
	return db.NewSelect().Model((*dbModels.NoteMediaAttachment)(nil)).Where("media_id = ?", id).Exists(ctx)
}

func (r *MediaRepo) AttachMediaToNote(ctx context.Context, tx *dbPorts.Tx, noteID string, mediaID string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewInsert().Model(&dbModels.NoteMediaAttachment{NoteID: noteID, MediaID: mediaID}).On("CONFLICT DO NOTHING").Exec(ctx)
	return err
}

func (r *MediaRepo) ListMediaForNote(ctx context.Context, tx *dbPorts.Tx, noteID string) ([]models.MediaAttachment, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var rows []dbModels.MediaAttachment
	err = db.NewSelect().Model(&rows).
		Join("JOIN note_media_attachments AS nma ON nma.media_id = media_attachment.id").
		Where("nma.note_id = ?", noteID).
		Order("nma.created_at ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]models.MediaAttachment, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.ToModel())
	}
	return res, nil
}
