package repos

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type CreateDomainBlockInput struct {
	Domain          string
	Severity        string
	RejectMedia     bool
	PublicComment   *string
	PrivateComment  *string
	CreatedByUserID string
}

type DomainBlocksRepository interface {
	CreateDomainBlock(ctx context.Context, tx *db.Tx, input CreateDomainBlockInput) (*models.DomainBlock, error)
	DeleteDomainBlock(ctx context.Context, tx *db.Tx, domain string) error
	ListDomainBlocks(ctx context.Context, tx *db.Tx) ([]models.DomainBlock, error)
	GetDomainBlock(ctx context.Context, tx *db.Tx, domain string) (*models.DomainBlock, error)
	DomainIsSuspended(ctx context.Context, tx *db.Tx, domain string) (bool, error)
}

type CreateModerationJobInput struct {
	Kind          string
	Payload       string
	NextAttemptAt time.Time
}

type ModerationJobsRepository interface {
	CreateModerationJob(ctx context.Context, tx *db.Tx, input CreateModerationJobInput) (*models.ModerationJob, error)
	ClaimDueModerationJobs(ctx context.Context, tx *db.Tx, now time.Time, limit int) ([]models.ModerationJob, error)
	MarkModerationJobDone(ctx context.Context, tx *db.Tx, id string, finishedAt time.Time) error
	MarkModerationJobFailed(ctx context.Context, tx *db.Tx, id string, attempts int, nextAttemptAt time.Time, lastError string) error
}

type DomainPurgeRepository interface {
	PurgeDomain(ctx context.Context, tx *db.Tx, domain string) (*models.PurgeDomainResult, []string, error)
}
