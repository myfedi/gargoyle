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
	row := &dbModels.MediaAttachment{ID: id, LocalAccountID: input.LocalAccountID, FileName: input.FileName, ContentType: input.ContentType, Data: input.Data, Description: input.Description}
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
