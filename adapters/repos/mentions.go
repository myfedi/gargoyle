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

type MentionsRepo struct{ db bun.IDB }

func NewMentionsRepo(db *bun.DB) *MentionsRepo { return &MentionsRepo{db: db} }

var _ repos.MentionsRepository = &MentionsRepo{}

func (r *MentionsRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New("internal error: unexpected tx implementation provided")
	}
	return adapted.Unwrap(), nil
}

func (r *MentionsRepo) CreateMention(ctx context.Context, tx *dbPorts.Tx, input repos.CreateMentionInput) (*models.Mention, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	row := &dbModels.Mention{ID: id, LocalAccountID: input.LocalAccountID, NoteID: input.NoteID, AccountID: input.AccountID, Username: input.Username, Acct: input.Acct, URL: input.URL}
	_, err = db.NewInsert().Model(row).On("CONFLICT DO NOTHING").Exec(ctx)
	if err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}

func (r *MentionsRepo) ListMentionsForNote(ctx context.Context, tx *dbPorts.Tx, noteID string) ([]models.Mention, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var rows []dbModels.Mention
	if err := db.NewSelect().Model(&rows).Where("note_id = ?", noteID).Order("created_at ASC").Scan(ctx); err != nil {
		return nil, err
	}
	res := make([]models.Mention, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.ToModel())
	}
	return res, nil
}
