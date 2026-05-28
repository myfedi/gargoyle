package models

import "time"

type StatusInteraction struct {
	ID             string
	CreatedAt      time.Time
	LocalAccountID string
	NoteID         string
	Type           string
}

type Notification struct {
	ID             string
	CreatedAt      time.Time
	LocalAccountID string
	ActorAccountID string
	Type           string
	StatusID       *string
	ReadAt         *time.Time
}
