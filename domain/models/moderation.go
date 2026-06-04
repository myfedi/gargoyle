package models

import "time"

const (
	DomainBlockSeveritySuspend   = "suspend"
	ModerationJobKindPurgeDomain = "purge_domain"
)

type DomainBlock struct {
	ID              string
	Domain          string
	Severity        string
	RejectMedia     bool
	PublicComment   *string
	PrivateComment  *string
	CreatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ModerationJob struct {
	ID            string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Kind          string
	Payload       string
	Attempts      int
	NextAttemptAt time.Time
	LastError     *string
	Status        JobStatus
	FinishedAt    *time.Time
}

type PurgeDomainResult struct {
	Domain                string
	DeletedNotes          int
	DeletedRemoteAccounts int
	DeletedFollows        int
	DeletedNotifications  int
	DeletedMedia          int
}
