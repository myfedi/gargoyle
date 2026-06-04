package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type DomainBlock struct {
	bun.BaseModel `bun:"table:domain_blocks"`

	ID              string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	Domain          string    `bun:",nullzero,notnull,unique"`
	Severity        string    `bun:",nullzero,notnull,default:'suspend'"`
	RejectMedia     bool      `bun:",notnull,default:true"`
	PublicComment   *string   `bun:",nullzero"`
	PrivateComment  *string   `bun:",nullzero"`
	CreatedByUserID string    `bun:"type:CHAR(26),nullzero,notnull"`
	CreatedAt       time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
}

func (d DomainBlock) ToModel() models.DomainBlock {
	return models.DomainBlock{ID: d.ID, Domain: d.Domain, Severity: d.Severity, RejectMedia: d.RejectMedia, PublicComment: d.PublicComment, PrivateComment: d.PrivateComment, CreatedByUserID: d.CreatedByUserID, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
}

type ModerationJob struct {
	bun.BaseModel `bun:"table:moderation_jobs"`

	ID            string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt     time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	Kind          string    `bun:",nullzero,notnull"`
	Payload       string    `bun:",nullzero,notnull"`
	Attempts      int       `bun:",notnull,default:0"`
	NextAttemptAt time.Time `bun:"type:timestamptz,nullzero,notnull"`
	LastError     *string
	Status        string     `bun:",nullzero,notnull,default:'pending'"`
	FinishedAt    *time.Time `bun:"type:timestamptz"`
}

func (j ModerationJob) ToModel() models.ModerationJob {
	return models.ModerationJob{ID: j.ID, CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt, Kind: j.Kind, Payload: j.Payload, Attempts: j.Attempts, NextAttemptAt: j.NextAttemptAt, LastError: j.LastError, Status: models.JobStatus(j.Status), FinishedAt: j.FinishedAt}
}
