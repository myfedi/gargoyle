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

type SocialRepo struct{ db bun.IDB }

func NewSocialRepo(db *bun.DB) *SocialRepo { return &SocialRepo{db: db} }

var _ repos.SocialRepository = &SocialRepo{}

func (r *SocialRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New("internal error: unexpected tx implementation provided")
	}
	return adapted.Unwrap(), nil
}

func (r *SocialRepo) CreateInteraction(ctx context.Context, tx *dbPorts.Tx, localAccountID string, noteID string, typ string) (*models.StatusInteraction, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	row := &dbModels.StatusInteraction{ID: id, LocalAccountID: localAccountID, NoteID: noteID, Type: typ}
	_, err = db.NewInsert().Model(row).On("CONFLICT DO NOTHING").Exec(ctx)
	if err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}
func (r *SocialRepo) DeleteInteraction(ctx context.Context, tx *dbPorts.Tx, localAccountID string, noteID string, typ string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.StatusInteraction)(nil)).Where("local_account_id = ?", localAccountID).Where("note_id = ?", noteID).Where("type = ?", typ).Exec(ctx)
	return err
}
func (r *SocialRepo) InteractionExists(ctx context.Context, tx *dbPorts.Tx, localAccountID string, noteID string, typ string) (bool, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return false, err
	}
	return db.NewSelect().Model((*dbModels.StatusInteraction)(nil)).Where("local_account_id = ?", localAccountID).Where("note_id = ?", noteID).Where("type = ?", typ).Exists(ctx)
}
func (r *SocialRepo) ListInteractions(ctx context.Context, tx *dbPorts.Tx, localAccountID string, typ string, limit int) ([]models.StatusInteraction, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	var rows []dbModels.StatusInteraction
	if err := db.NewSelect().Model(&rows).Where("local_account_id = ?", localAccountID).Where("type = ?", typ).Order("created_at DESC").Limit(limit).Scan(ctx); err != nil {
		return nil, err
	}
	res := make([]models.StatusInteraction, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.ToModel())
	}
	return res, nil
}

func (r *SocialRepo) CreateNotification(ctx context.Context, tx *dbPorts.Tx, localAccountID string, actorAccountID string, typ string, statusID *string) (*models.Notification, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	row := &dbModels.Notification{ID: id, LocalAccountID: localAccountID, ActorAccountID: actorAccountID, Type: typ, StatusID: statusID}
	_, err = db.NewInsert().Model(row).Exec(ctx)
	if err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}
func (r *SocialRepo) ListNotifications(ctx context.Context, tx *dbPorts.Tx, localAccountID string, limit int) ([]models.Notification, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	var rows []dbModels.Notification
	if err := db.NewSelect().Model(&rows).Where("local_account_id = ?", localAccountID).Order("created_at DESC").Limit(limit).Scan(ctx); err != nil {
		return nil, err
	}
	res := make([]models.Notification, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.ToModel())
	}
	return res, nil
}
func (r *SocialRepo) DeleteNotification(ctx context.Context, tx *dbPorts.Tx, localAccountID string, notificationID string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.Notification)(nil)).Where("local_account_id = ?", localAccountID).Where("id = ?", notificationID).Exec(ctx)
	return err
}

func (r *SocialRepo) ClearNotifications(ctx context.Context, tx *dbPorts.Tx, localAccountID string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.Notification)(nil)).Where("local_account_id = ?", localAccountID).Exec(ctx)
	return err
}
