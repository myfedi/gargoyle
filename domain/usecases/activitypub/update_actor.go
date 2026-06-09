package activitypub

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// UpdateActorInput contains AP command data for updating a local actor profile.
type UpdateActorInput struct {
	Username      string
	UpdateID      string
	DisplayName   *string
	Summary       *string
	Fields        []models.AccountProfileField
	AvatarMediaID *string
	HeaderMediaID *string
	Locked        *bool
}

// UpdateActorResult contains the committed Update payload and delivery data.
type UpdateActorResult struct {
	Account         models.Account
	RawJSON         []byte
	FollowerInboxes []string
}

// UpdateActor updates a local actor profile and stores an outbound Update activity in one transaction.
func (u *UpdateActorUseCase) UpdateActor(ctx context.Context, input UpdateActorInput) (*UpdateActorResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, input.Username)
	if derr != nil {
		return nil, derr
	}
	if account.UserID == nil {
		return nil, domainerrors.New(domainerrors.ErrUnauthorized, "only local accounts can update credentials")
	}
	if input.UpdateID == "" {
		return nil, domainerrors.New(domainerrors.ErrInternal, "update id is required")
	}

	var updated *models.Account
	var raw []byte
	err := u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		var err error
		updated, err = u.cfg.AccountsRepo.UpdateLocalAccountProfile(ctx, &tx, account.ID, repos.UpdateAccountProfileInput{DisplayName: input.DisplayName, Summary: input.Summary, Fields: input.Fields, AvatarMediaID: input.AvatarMediaID, HeaderMediaID: input.HeaderMediaID, AvatarURL: nil, HeaderURL: nil, Locked: input.Locked})
		if err != nil {
			return err
		}
		profileRaw, derr := u.actorUpdateActivity(*updated, input.UpdateID)
		if derr != nil {
			return derr
		}
		raw = profileRaw
		_, err = u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: updated.ID, Direction: models.ActivityDirectionOutbox, Type: "Update", Actor: updated.URI, Object: updated.URI, RawJSON: string(raw)})
		return err
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	inboxes, derr := updateActorFollowerInboxes(ctx, u.cfg.FollowsRepo, updated.ID)
	if derr != nil {
		return nil, derr
	}
	return &UpdateActorResult{Account: *updated, RawJSON: raw, FollowerInboxes: inboxes}, nil
}

func (u *UpdateActorUseCase) actorUpdateActivity(account models.Account, updateID string) ([]byte, *domainerrors.DomainError) {
	actorJSON, err := u.cfg.ActorSerializer.Marshall(account)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	var actor map[string]any
	if err := json.Unmarshal([]byte(actorJSON), &actor); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	delete(actor, activityStreamsContextKey)
	activity := map[string]any{
		activityStreamsContextKey: activityStreamsContextURI,
		"id":                      strings.TrimRight(account.URI, "/") + "/updates/" + updateID,
		"type":                    "Update",
		"actor":                   account.URI,
		"published":               time.Now().UTC().Format(time.RFC3339),
		"to":                      []string{activityStreamsPublicURI},
		"cc":                      []string{account.FollowersURI},
		"object":                  actor,
	}
	raw, err := json.Marshal(activity)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return raw, nil
}

func updateActorFollowerInboxes(ctx context.Context, repo repos.FollowsRepository, accountID string) ([]string, *domainerrors.DomainError) {
	followers, err := repo.ListFollowers(ctx, nil, accountID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	inboxes := make([]string, 0, len(followers))
	for _, follower := range followers {
		if follower.RemoteInbox != nil {
			inboxes = append(inboxes, *follower.RemoteInbox)
		}
	}
	return inboxes, nil
}
