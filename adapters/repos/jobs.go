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

type JobsRepo struct{ db bun.IDB }

func NewJobsRepo(db *bun.DB) *JobsRepo { return &JobsRepo{db: db} }

var _ repos.DeliveryJobsRepository = &JobsRepo{}
var _ repos.FetchJobsRepository = &JobsRepo{}

func (r *JobsRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New("internal error: unexpected tx implementation provided")
	}
	return adapted.Unwrap(), nil
}

func (r *JobsRepo) CreateDeliveryJob(ctx context.Context, tx *dbPorts.Tx, input repos.CreateDeliveryJobInput) (*models.DeliveryJob, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	job := &dbModels.DeliveryJob{ID: id, AccountID: input.AccountID, ActivityID: input.ActivityID, InboxURL: input.InboxURL, Payload: input.Payload, NextAttemptAt: input.NextAttemptAt, Status: string(models.JobStatusPending)}
	if _, err := db.NewInsert().Model(job).Exec(ctx); err != nil {
		return nil, err
	}
	model := job.ToModel()
	return &model, nil
}

func (r *JobsRepo) ListDueDeliveryJobs(ctx context.Context, tx *dbPorts.Tx, now time.Time, limit int) ([]models.DeliveryJob, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var rows []dbModels.DeliveryJob
	query := db.NewSelect().Model(&rows).Where("status = ?", string(models.JobStatusPending)).Where("next_attempt_at <= ?", now).Order("next_attempt_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, err
	}
	jobs := make([]models.DeliveryJob, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, row.ToModel())
	}
	return jobs, nil
}

func (r *JobsRepo) CreateFetchJob(ctx context.Context, tx *dbPorts.Tx, input repos.CreateFetchJobInput) (*models.FetchJob, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	job := &dbModels.FetchJob{ID: id, URL: input.URL, Kind: input.Kind, NextAttemptAt: input.NextAttemptAt, Status: string(models.JobStatusPending)}
	if _, err := db.NewInsert().Model(job).Exec(ctx); err != nil {
		return nil, err
	}
	model := job.ToModel()
	return &model, nil
}

func (r *JobsRepo) ListDueFetchJobs(ctx context.Context, tx *dbPorts.Tx, now time.Time, limit int) ([]models.FetchJob, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var rows []dbModels.FetchJob
	query := db.NewSelect().Model(&rows).Where("status = ?", string(models.JobStatusPending)).Where("next_attempt_at <= ?", now).Order("next_attempt_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, err
	}
	jobs := make([]models.FetchJob, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, row.ToModel())
	}
	return jobs, nil
}
