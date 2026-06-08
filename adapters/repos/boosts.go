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

type BoostsRepo struct{ db bun.IDB }

func NewBoostsRepo(db *bun.DB) *BoostsRepo { return &BoostsRepo{db: db} }

var _ repos.BoostsRepository = &BoostsRepo{}

func (r *BoostsRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New(unexpectedTxImplementationError)
	}
	return adapted.Unwrap(), nil
}

func (r *BoostsRepo) CreateBoost(ctx context.Context, tx *dbPorts.Tx, input repos.CreateBoostInput) (*models.Boost, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	row := &dbModels.Boost{ID: id, LocalAccountID: input.LocalAccountID, Actor: input.Actor, NoteID: input.NoteID, URI: input.URI, PublishedAt: input.PublishedAt}
	_, err = db.NewInsert().Model(row).On("CONFLICT (local_account_id, actor, note_id) DO UPDATE").Set("published_at = EXCLUDED.published_at").Set("uri = EXCLUDED.uri").Exec(ctx)
	if err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}
func (r *BoostsRepo) DeleteBoost(ctx context.Context, tx *dbPorts.Tx, localAccountID, actor, noteID string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.Boost)(nil)).Where("local_account_id = ?", localAccountID).Where("actor = ?", actor).Where("note_id = ?", noteID).Exec(ctx) // NOSONAR
	return err
}
func (r *BoostsRepo) ListTimelineBoosts(ctx context.Context, tx *dbPorts.Tx, localAccountID string, limit int, maxID string) ([]models.Boost, error) {
	return r.listBoosts(ctx, tx, localAccountID, "", limit, maxID)
}
func (r *BoostsRepo) ListActorBoosts(ctx context.Context, tx *dbPorts.Tx, localAccountID, actor string, limit int, maxID string) ([]models.Boost, error) {
	return r.listBoosts(ctx, tx, localAccountID, actor, limit, maxID)
}
func (r *BoostsRepo) listBoosts(ctx context.Context, tx *dbPorts.Tx, localAccountID, actor string, limit int, maxID string) ([]models.Boost, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	var rows []dbModels.Boost
	q := db.NewSelect().Model(&rows).Where("local_account_id = ?", localAccountID).Order("published_at DESC", "id DESC").Limit(limit) // NOSONAR
	if actor != "" {
		q = q.Where("actor = ?", actor) // NOSONAR
	}
	if maxID != "" {
		var cursor dbModels.Boost
		if err := db.NewSelect().Model(&cursor).Where("id = ?", maxID).Limit(1).Scan(ctx); err == nil {
			q = q.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
				return q.Where("published_at < ?", cursor.PublishedAt).WhereOr("published_at = ? AND id < ?", cursor.PublishedAt, maxID)
			})
		} else {
			var noteCursor dbModels.Note
			if err := db.NewSelect().Model(&noteCursor).Where("id = ?", maxID).Limit(1).Scan(ctx); err == nil {
				q = q.Where("published_at < ?", noteCursor.PublishedAt)
			} else {
				q = q.Where("id < ?", maxID)
			}
		}
	}
	if err := q.Scan(ctx); err != nil {
		return nil, err
	}
	res := make([]models.Boost, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.ToModel())
	}
	return res, nil
}
func (r *BoostsRepo) CountBoostsForNote(ctx context.Context, tx *dbPorts.Tx, noteID string) (int, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return 0, err
	}
	return db.NewSelect().Model((*dbModels.Boost)(nil)).Where("note_id = ?", noteID).Count(ctx) // NOSONAR
}
func (r *BoostsRepo) BoostExists(ctx context.Context, tx *dbPorts.Tx, localAccountID, actor, noteID string) (bool, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return false, err
	}
	return db.NewSelect().Model((*dbModels.Boost)(nil)).Where("local_account_id = ?", localAccountID).Where("actor = ?", actor).Where("note_id = ?", noteID).Exists(ctx) // NOSONAR
}
