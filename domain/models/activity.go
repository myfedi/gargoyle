package models

import "time"

type ActivityDirection string

const (
	ActivityDirectionInbox  ActivityDirection = "inbox"
	ActivityDirectionOutbox ActivityDirection = "outbox"
)

type Activity struct {
	ID             string
	LocalAccountID string
	Direction      ActivityDirection
	Type           string
	Actor          string
	Object         string
	RawJSON        string
	CreatedAt      time.Time
}

type Follow struct {
	ID             string
	LocalAccountID string
	RemoteActor    string
	RemoteInbox    *string
	ActivityID     string
	AcceptedAt     *time.Time
	CreatedAt      time.Time
}
