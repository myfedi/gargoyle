package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

type PollVoteDelivery struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

type VotePollResult struct {
	Poll       models.Poll
	Deliveries []PollVoteDelivery
}

func (u Interactions) VotePoll(ctx context.Context, account *models.Account, pollID string, choices []int) (*VotePollResult, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	voteIDs := make([]string, 0, len(choices))
	for range choices {
		id, err := u.deps.IDGenerator.NewID()
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		voteIDs = append(voteIDs, id)
	}
	res, derr := u.deps.VotePollUC.VotePoll(ctx, apUsecases.VotePollInput{Username: account.Username, PollID: pollID, Choices: choices, VoteIDs: voteIDs})
	if derr != nil {
		return nil, derr
	}
	deliveries := make([]PollVoteDelivery, 0, len(res.Deliveries))
	for _, delivery := range res.Deliveries {
		deliveries = append(deliveries, PollVoteDelivery{Account: delivery.Account, RawJSON: delivery.RawJSON, Inbox: delivery.Inbox})
	}
	return &VotePollResult{Poll: res.Poll, Deliveries: deliveries}, nil
}

func (u Interactions) pollForNote(ctx context.Context, account *models.Account, note models.Note) (*models.Poll, *domainerrors.DomainError) {
	if note.ObjectType != "Question" {
		return nil, nil
	}
	options, err := u.deps.PollsRepo.GetPollOptions(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if len(options) == 0 {
		return nil, nil
	}
	ownVotes, err := u.deps.PollsRepo.LocalVoteChoices(ctx, nil, note.ID, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &models.Poll{NoteID: note.ID, Options: options, Multiple: note.PollMultiple, ExpiresAt: note.PollExpiresAt, Voted: len(ownVotes) > 0, OwnVotes: ownVotes}, nil
}
