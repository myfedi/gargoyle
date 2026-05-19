package models

import "time"

type Note struct {
	ID             string
	LocalAccountID string
	ActivityID     string
	URI            string
	Content        string
	PlainText      string
	AttributedTo   string
	PublishedAt    time.Time
	CreatedAt      time.Time
}
