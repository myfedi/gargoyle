package models

import "time"

type MediaAttachment struct {
	ID             string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LocalAccountID string
	FileName       string
	ContentType    string
	Data           []byte
	Description    string
}
