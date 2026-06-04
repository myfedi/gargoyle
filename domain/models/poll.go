package models

import "time"

type PollOption struct {
	ID         string
	NoteID     string
	Title      string
	Position   int
	VotesCount int
	CreatedAt  time.Time
}

type PollVote struct {
	ID             string
	PollOptionID   string
	LocalAccountID *string
	RemoteActor    *string
	CreatedAt      time.Time
}

type Poll struct {
	NoteID    string
	Options   []PollOption
	Multiple  bool
	ExpiresAt *time.Time
	Voted     bool
	OwnVotes  []int
}
