package repos

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type CreateDeliveryJobInput struct {
	AccountID     string
	ActivityID    string
	InboxURL      string
	Payload       []byte
	NextAttemptAt time.Time
}

type DeliveryJobsRepository interface {
	CreateDeliveryJob(ctx context.Context, tx *db.Tx, input CreateDeliveryJobInput) (*models.DeliveryJob, error)
	ListDueDeliveryJobs(ctx context.Context, tx *db.Tx, now time.Time, limit int) ([]models.DeliveryJob, error)
	MarkDeliveryJobDelivered(ctx context.Context, tx *db.Tx, id string, deliveredAt time.Time) error
	MarkDeliveryJobFailed(ctx context.Context, tx *db.Tx, id string, attempts int, nextAttemptAt time.Time, lastError string) error
}

type CreateFetchJobInput struct {
	URL           string
	Kind          string
	NextAttemptAt time.Time
}

type FetchJobsRepository interface {
	CreateFetchJob(ctx context.Context, tx *db.Tx, input CreateFetchJobInput) (*models.FetchJob, error)
	ListDueFetchJobs(ctx context.Context, tx *db.Tx, now time.Time, limit int) ([]models.FetchJob, error)
	MarkFetchJobFetched(ctx context.Context, tx *db.Tx, id string, fetchedAt time.Time) error
	MarkFetchJobFailed(ctx context.Context, tx *db.Tx, id string, attempts int, nextAttemptAt time.Time, lastError string) error
}
