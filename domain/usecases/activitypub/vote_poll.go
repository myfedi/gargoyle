package activitypub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

// PollVoteDelivery contains a committed AP Create vote payload and remote inbox.
type PollVoteDelivery struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

// VotePollInput contains AP command data for voting on an ActivityPub Question.
type VotePollInput struct {
	Username string
	PollID   string
	Choices  []int
	VoteIDs  []string
}

// VotePollResult contains stored poll state and optional vote deliveries.
type VotePollResult struct {
	Poll       models.Poll
	Deliveries []PollVoteDelivery
}

func (u *VotePollUseCase) VotePoll(ctx context.Context, input VotePollInput) (*VotePollResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, input.Username)
	if derr != nil {
		return nil, derr
	}
	note, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, input.PollID)
	if err != nil || note.ObjectType != "Question" {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "poll not found")
	}
	if note.PollExpiresAt != nil && time.Now().UTC().After(*note.PollExpiresAt) {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "poll has expired")
	}
	if len(input.Choices) == 0 {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "at least one poll choice is required")
	}
	if note.AttributedTo != account.URI && len(input.VoteIDs) < len(input.Choices) {
		return nil, domainerrors.New(domainerrors.ErrInternal, "not enough vote ids")
	}

	var options []models.PollOption
	err = u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		stored, err := u.cfg.PollsRepo.CreateLocalVote(ctx, &tx, note.ID, account.ID, input.Choices, note.PollMultiple)
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
	deliveries, derr := u.pollVoteDeliveries(ctx, *account, *note, options, input.Choices, input.VoteIDs)
	if derr != nil {
		return nil, derr
	}
	return &VotePollResult{Poll: models.Poll{NoteID: note.ID, Options: options, Multiple: note.PollMultiple, ExpiresAt: note.PollExpiresAt, Voted: true, OwnVotes: input.Choices}, Deliveries: deliveries}, nil
}

func (u *VotePollUseCase) pollVoteDeliveries(ctx context.Context, account models.Account, note models.Note, options []models.PollOption, choices []int, voteIDs []string) ([]PollVoteDelivery, *domainerrors.DomainError) {
	inbox := u.remotePollAuthorInbox(ctx, account, note.AttributedTo)
	if inbox == "" {
		return nil, nil
	}
	byPosition := map[int]models.PollOption{}
	for _, option := range options {
		byPosition[option.Position] = option
	}
	deliveries := make([]PollVoteDelivery, 0, len(choices))
	for index, choice := range choices {
		option, ok := byPosition[choice]
		if !ok {
			continue
		}
		id := voteIDs[index]
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
		deliveries = append(deliveries, PollVoteDelivery{Account: account, RawJSON: raw, Inbox: inbox})
	}
	return deliveries, nil
}

const remotePollAuthorRefreshAfter = 24 * time.Hour

func (u *VotePollUseCase) remotePollAuthorInbox(ctx context.Context, signer models.Account, actor string) string {
	if actor == "" || actor == signer.URI || strings.HasPrefix(actor, strings.TrimRight(u.cfg.Host, "/")+"/users/") {
		return ""
	}
	var cached *models.Account
	if u.cfg.RemoteAccountsRepo != nil {
		if remote, err := u.cfg.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, actor); err == nil {
			cached = remote
			if remote.InboxURI != "" && time.Since(remote.FetchedAt) < remotePollAuthorRefreshAfter {
				return remote.InboxURI
			}
		}
	}
	if u.cfg.ActorFetcher != nil {
		fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if doc, err := u.cfg.ActorFetcher.FetchActor(fetchCtx, actor, &signer); err == nil && doc != nil && doc.Inbox != "" {
			return doc.Inbox
		}
	}
	if cached != nil {
		return cached.InboxURI
	}
	return ""
}
