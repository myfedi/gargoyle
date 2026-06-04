package models

import "time"

type Note struct {
	ID             string
	LocalAccountID string
	ActivityID     string
	URI            string
	Content        string
	PlainText      string
	ObjectType     string
	Visibility     string
	PollMultiple   bool
	PollExpiresAt  *time.Time
	Hashtags       []string
	Emojis         []CustomEmoji
	Sensitive      bool
	SpoilerText    string
	AttributedTo   string
	InReplyToID    *string
	InReplyToURI   *string
	PublishedAt    time.Time
	CreatedAt      time.Time
	EditedAt       *time.Time
}

type NoteEdit struct {
	ID          string
	NoteID      string
	Content     string
	PlainText   string
	Visibility  string
	Sensitive   bool
	SpoilerText string
	ObjectType  string
	CreatedAt   time.Time
	MediaIDs    []string
}
