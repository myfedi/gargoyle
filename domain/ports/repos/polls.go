package repos

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type CreatePollInput struct {
	NoteID   string
	Options  []string
	Multiple bool
}

type PollsRepository interface {
	CreatePoll(ctx context.Context, tx *db.Tx, input CreatePollInput) ([]models.PollOption, error)
	ReplacePoll(ctx context.Context, tx *db.Tx, input CreatePollInput) ([]models.PollOption, error)
	GetPollOptions(ctx context.Context, tx *db.Tx, noteID string) ([]models.PollOption, error)
	CreateLocalVote(ctx context.Context, tx *db.Tx, noteID, localAccountID string, choices []int, multiple bool) ([]models.PollOption, error)
	CreateRemoteVote(ctx context.Context, tx *db.Tx, noteID, remoteActor, optionTitle string, multiple bool) ([]models.PollOption, error)
	LocalVoteChoices(ctx context.Context, tx *db.Tx, noteID, localAccountID string) ([]int, error)
}
