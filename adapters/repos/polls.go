package repos

import (
	"context"
	"database/sql"
	"errors"

	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	"github.com/myfedi/gargoyle/domain/models"
	dbPorts "github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	dbUtils "github.com/myfedi/gargoyle/infrastructure/db"
	dbModels "github.com/myfedi/gargoyle/infrastructure/db/models"
	"github.com/uptrace/bun"
)

type PollsRepo struct{ db bun.IDB }

func NewPollsRepo(db *bun.DB) *PollsRepo { return &PollsRepo{db: db} }

var _ repos.PollsRepository = &PollsRepo{}

func (r *PollsRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New(unexpectedTxImplementationError)
	}
	return adapted.Unwrap(), nil
}

func (r *PollsRepo) CreatePoll(ctx context.Context, tx *dbPorts.Tx, input repos.CreatePollInput) ([]models.PollOption, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	return insertPollOptions(ctx, db, input.NoteID, input.Options)
}

func (r *PollsRepo) ReplacePoll(ctx context.Context, tx *dbPorts.Tx, input repos.CreatePollInput) ([]models.PollOption, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	if _, err := db.NewDelete().Model((*dbModels.PollVote)(nil)).Where("note_id = ?", input.NoteID).Exec(ctx); err != nil { // NOSONAR
		return nil, err
	}
	if _, err := db.NewDelete().Model((*dbModels.PollOption)(nil)).Where("note_id = ?", input.NoteID).Exec(ctx); err != nil { // NOSONAR
		return nil, err
	}
	return insertPollOptions(ctx, db, input.NoteID, input.Options)
}

func insertPollOptions(ctx context.Context, db bun.IDB, noteID string, titles []string) ([]models.PollOption, error) {
	res := make([]models.PollOption, 0, len(titles))
	for i, title := range titles {
		id, err := dbUtils.NewULID()
		if err != nil {
			return nil, err
		}
		row := &dbModels.PollOption{ID: id, NoteID: noteID, Title: title, Position: i}
		if _, err := db.NewInsert().Model(row).Exec(ctx); err != nil {
			return nil, err
		}
		res = append(res, row.ToModel())
	}
	return res, nil
}

func (r *PollsRepo) GetPollOptions(ctx context.Context, tx *dbPorts.Tx, noteID string) ([]models.PollOption, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var rows []dbModels.PollOption
	if err := db.NewSelect().Model(&rows).Where("note_id = ?", noteID).Order("position ASC").Scan(ctx); err != nil { // NOSONAR
		return nil, err
	}
	res := make([]models.PollOption, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.ToModel())
	}
	return res, nil
}

func (r *PollsRepo) CreateLocalVote(ctx context.Context, tx *dbPorts.Tx, noteID, localAccountID string, choices []int, multiple bool) ([]models.PollOption, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	choices = uniqueChoices(choices)
	if !multiple && len(choices) > 1 {
		return nil, errors.New("poll allows one choice")
	}
	options, err := r.GetPollOptions(ctx, tx, noteID)
	if err != nil {
		return nil, err
	}
	byPosition := map[int]models.PollOption{}
	for _, option := range options {
		byPosition[option.Position] = option
	}
	if _, err := db.NewDelete().Model((*dbModels.PollVote)(nil)).Where("note_id = ?", noteID).Where("local_account_id = ?", localAccountID).Exec(ctx); err != nil { // NOSONAR
		return nil, err
	}
	for _, choice := range choices {
		option, ok := byPosition[choice]
		if !ok {
			return nil, sql.ErrNoRows
		}
		id, err := dbUtils.NewULID()
		if err != nil {
			return nil, err
		}
		vote := &dbModels.PollVote{ID: id, NoteID: noteID, PollOptionID: option.ID, LocalAccountID: &localAccountID}
		if _, err := db.NewInsert().Model(vote).Exec(ctx); err != nil {
			return nil, err
		}
	}
	if _, err := db.NewUpdate().Model((*dbModels.PollOption)(nil)).Set("votes_count = (SELECT COUNT(*) FROM poll_votes WHERE poll_option_id = poll_option.id)").Where("note_id = ?", noteID).Exec(ctx); err != nil { // NOSONAR
		return nil, err
	}
	return r.GetPollOptions(ctx, tx, noteID)
}

func (r *PollsRepo) CreateRemoteVote(ctx context.Context, tx *dbPorts.Tx, noteID, remoteActor, optionTitle string, multiple bool) ([]models.PollOption, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	options, err := r.GetPollOptions(ctx, tx, noteID)
	if err != nil {
		return nil, err
	}
	var selected *models.PollOption
	for _, option := range options {
		if option.Title == optionTitle {
			copy := option
			selected = &copy
			break
		}
	}
	if selected == nil {
		return nil, sql.ErrNoRows
	}
	if !multiple {
		if _, err := db.NewDelete().Model((*dbModels.PollVote)(nil)).Where("note_id = ?", noteID).Where("remote_actor = ?", remoteActor).Exec(ctx); err != nil { // NOSONAR
			return nil, err
		}
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	vote := &dbModels.PollVote{ID: id, NoteID: noteID, PollOptionID: selected.ID, RemoteActor: &remoteActor}
	if _, err := db.NewInsert().Model(vote).On("CONFLICT DO NOTHING").Exec(ctx); err != nil {
		return nil, err
	}
	if _, err := db.NewUpdate().Model((*dbModels.PollOption)(nil)).Set("votes_count = (SELECT COUNT(*) FROM poll_votes WHERE poll_option_id = poll_option.id)").Where("note_id = ?", noteID).Exec(ctx); err != nil { // NOSONAR
		return nil, err
	}
	return r.GetPollOptions(ctx, tx, noteID)
}

func uniqueChoices(choices []int) []int {
	seen := map[int]bool{}
	res := make([]int, 0, len(choices))
	for _, choice := range choices {
		if seen[choice] {
			continue
		}
		seen[choice] = true
		res = append(res, choice)
	}
	return res
}

func (r *PollsRepo) LocalVoteChoices(ctx context.Context, tx *dbPorts.Tx, noteID, localAccountID string) ([]int, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var rows []dbModels.PollOption
	if err := db.NewSelect().Model(&rows).
		Join("JOIN poll_votes AS vote ON vote.poll_option_id = poll_option.id").
		Where("poll_option.note_id = ?", noteID).
		Where("vote.local_account_id = ?", localAccountID).
		Order("poll_option.position ASC").
		Scan(ctx); err != nil {
		return nil, err
	}
	choices := make([]int, 0, len(rows))
	for _, row := range rows {
		choices = append(choices, row.Position)
	}
	return choices, nil
}
