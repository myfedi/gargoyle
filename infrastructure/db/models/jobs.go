package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type DeliveryJob struct {
	bun.BaseModel `bun:"table:delivery_jobs"`

	ID            string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt     time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	AccountID     string    `bun:"type:CHAR(26),nullzero,notnull"`
	ActivityID    string
	InboxURL      string    `bun:",nullzero,notnull"`
	Payload       []byte    `bun:",nullzero,notnull"`
	Attempts      int       `bun:",notnull,default:0"`
	NextAttemptAt time.Time `bun:"type:timestamptz,nullzero,notnull"`
	LastError     *string
	Status        string     `bun:",nullzero,notnull,default:'pending'"`
	DeliveredAt   *time.Time `bun:"type:timestamptz"`
}

func (j DeliveryJob) ToModel() models.DeliveryJob {
	return models.DeliveryJob{ID: j.ID, CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt, AccountID: j.AccountID, ActivityID: j.ActivityID, InboxURL: j.InboxURL, Payload: j.Payload, Attempts: j.Attempts, NextAttemptAt: j.NextAttemptAt, LastError: j.LastError, Status: models.JobStatus(j.Status), DeliveredAt: j.DeliveredAt}
}

type FetchJob struct {
	bun.BaseModel `bun:"table:fetch_jobs"`

	ID            string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt     time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	URL           string    `bun:",nullzero,notnull"`
	Kind          string    `bun:",nullzero,notnull"`
	AccountID     string    `bun:"type:CHAR(26),nullzero,notnull"`
	Attempts      int       `bun:",notnull,default:0"`
	NextAttemptAt time.Time `bun:"type:timestamptz,nullzero,notnull"`
	LastError     *string
	Status        string     `bun:",nullzero,notnull,default:'pending'"`
	FetchedAt     *time.Time `bun:"type:timestamptz"`
}

func (j FetchJob) ToModel() models.FetchJob {
	return models.FetchJob{ID: j.ID, CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt, URL: j.URL, Kind: j.Kind, AccountID: j.AccountID, Attempts: j.Attempts, NextAttemptAt: j.NextAttemptAt, LastError: j.LastError, Status: models.JobStatus(j.Status), FetchedAt: j.FetchedAt}
}
