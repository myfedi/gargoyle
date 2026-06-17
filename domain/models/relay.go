package models

import "time"

const (
	RelayStatusPending  = "pending"
	RelayStatusAccepted = "accepted"
	RelayStatusDisabled = "disabled"
)

type RelaySubscription struct {
	ID              string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ActorURI        string
	InboxURI        string
	Status          string
	AcceptedAt      *time.Time
	CreatedByUserID string
	LastError       *string
}
