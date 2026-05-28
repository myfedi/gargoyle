package models

import "time"

type Boost struct {
	ID             string
	CreatedAt      time.Time
	LocalAccountID string
	Actor          string
	NoteID         string
	URI            string
	PublishedAt    time.Time
}
