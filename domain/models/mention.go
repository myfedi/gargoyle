package models

import "time"

type Mention struct {
	ID             string
	CreatedAt      time.Time
	LocalAccountID string
	NoteID         string
	AccountID      string
	Username       string
	Acct           string
	URL            string
}
