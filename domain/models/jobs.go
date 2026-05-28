package models

import "time"

type JobStatus string

const (
	JobStatusPending JobStatus = "pending"
	JobStatusRunning JobStatus = "running"
	JobStatusDone    JobStatus = "done"
	JobStatusFailed  JobStatus = "failed"
)

type DeliveryJob struct {
	ID            string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	AccountID     string
	ActivityID    string
	InboxURL      string
	Payload       []byte
	Attempts      int
	NextAttemptAt time.Time
	LastError     *string
	Status        JobStatus
	DeliveredAt   *time.Time
}

type FetchJob struct {
	ID            string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	URL           string
	Kind          string
	AccountID     string
	Attempts      int
	NextAttemptAt time.Time
	LastError     *string
	Status        JobStatus
	FetchedAt     *time.Time
}
