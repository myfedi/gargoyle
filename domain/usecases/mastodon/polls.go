package mastodon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
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

func (u UseCase) VotePoll(ctx context.Context, account *models.Account, pollID string, choices []int) (*VotePollResult, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	note, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, pollID)
	if err != nil || note.ObjectType != "Question" {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "poll not found")
	}
	if note.PollExpiresAt != nil && time.Now().UTC().After(*note.PollExpiresAt) {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "poll has expired")
	}
	if len(choices) == 0 {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "at least one poll choice is required")
	}
	var options []models.PollOption
	err = u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		stored, err := u.cfg.PollsRepo.CreateLocalVote(ctx, &tx, note.ID, account.ID, choices, note.PollMultiple)
		if err != nil {
			return err
		}
		options = stored
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainerrors.New(domainerrors.ErrBadRequest, "poll choice is invalid")
		}
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	deliveries, derr := u.pollVoteDeliveries(ctx, account, *note, options, choices)
	if derr != nil {
		return nil, derr
	}
	return &VotePollResult{Poll: models.Poll{NoteID: note.ID, Options: options, Multiple: note.PollMultiple, ExpiresAt: note.PollExpiresAt, Voted: true, OwnVotes: choices}, Deliveries: deliveries}, nil
}

func (u UseCase) pollVoteDeliveries(ctx context.Context, account *models.Account, note models.Note, options []models.PollOption, choices []int) ([]PollVoteDelivery, *domainerrors.DomainError) {
	if note.AttributedTo == account.URI {
		return nil, nil
	}
	author, derr := u.accountForActor(ctx, account, note.AttributedTo)
	if derr != nil {
		return nil, derr
	}
	if author.InboxURI == "" {
		return nil, nil
	}
	byPosition := map[int]models.PollOption{}
	for _, option := range options {
		byPosition[option.Position] = option
	}
	deliveries := make([]PollVoteDelivery, 0, len(choices))
	for _, choice := range choices {
		option, ok := byPosition[choice]
		if !ok {
			continue
		}
		id, err := u.cfg.IDGenerator.NewID()
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		activity := map[string]any{
			activityStreamsContextKey: activityStreamsContextURI,
			"id":                      account.URI + "/votes/" + id,
			"type":                    "Create",
			"actor":                   account.URI,
			"to":                      []string{note.AttributedTo},
			"object": map[string]any{
				"id":           account.URI + "/votes/" + id + "/object",
				"type":         "Note",
				"name":         option.Title,
				"attributedTo": account.URI,
				"inReplyTo":    note.URI,
				"to":           []string{note.AttributedTo},
			},
		}
		raw, err := json.Marshal(activity)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		deliveries = append(deliveries, PollVoteDelivery{Account: *account, RawJSON: raw, Inbox: author.InboxURI})
	}
	return deliveries, nil
}

func (u UseCase) pollForNote(ctx context.Context, account *models.Account, note models.Note) (*models.Poll, *domainerrors.DomainError) {
	if note.ObjectType != "Question" {
		return nil, nil
	}
	options, err := u.cfg.PollsRepo.GetPollOptions(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if len(options) == 0 {
		return nil, nil
	}
	ownVotes, err := u.cfg.PollsRepo.LocalVoteChoices(ctx, nil, note.ID, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &models.Poll{NoteID: note.ID, Options: options, Multiple: note.PollMultiple, ExpiresAt: note.PollExpiresAt, Voted: len(ownVotes) > 0, OwnVotes: ownVotes}, nil
}
