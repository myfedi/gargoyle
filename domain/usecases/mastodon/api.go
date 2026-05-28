package mastodon

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

// Config wires the Mastodon-compatible client API use case to domain ports and
// lower-level ActivityPub workflows.
type Config struct {
	Host           string
	Domain         string
	ServerVersion  string
	NotesRepo      repos.NotesRepository
	IDGenerator    ports.IDGenerator
	CreateOutboxUC apUsecases.CreateOutboxActivityUseCase
}

// UseCase owns client-facing Mastodon API workflows. HTTP handlers should only
// authenticate, parse requests, and serialize responses.
type UseCase struct{ cfg Config }

type InstanceInfo struct {
	Host          string
	Domain        string
	Title         string
	Description   string
	ServerVersion string
}

func NewUseCase(cfg Config) UseCase {
	if cfg.Host == "" || cfg.Domain == "" {
		panic("mastodon API use case requires Host and Domain")
	}
	if cfg.NotesRepo == nil {
		panic("mastodon API use case requires NotesRepo")
	}
	if cfg.IDGenerator == nil {
		panic("mastodon API use case requires IDGenerator")
	}
	return UseCase{cfg: cfg}
}

func (u UseCase) InstanceInfo() InstanceInfo {
	return InstanceInfo{Host: u.cfg.Host, Domain: u.cfg.Domain, Title: "Gargoyle", Description: "Gargoyle federated server", ServerVersion: u.cfg.ServerVersion}
}

func (u UseCase) CreateStatus(ctx context.Context, account *models.Account, content string) (*models.Note, *domainerrors.DomainError) {
	if account == nil {
		return nil, domainerrors.New(domainerrors.ErrUnauthorized, "missing account")
	}
	if strings.TrimSpace(content) == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "status is required")
	}
	activityID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	objectID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	raw, err := json.Marshal(map[string]any{"type": "Note", "content": content})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res, derr := u.cfg.CreateOutboxUC.CreateOutboxActivity(ctx, apUsecases.CreateOutboxActivityInput{Username: account.Username, RawJSON: raw, ActivityID: activityID, ObjectID: objectID})
	if derr != nil {
		return nil, derr
	}
	extracted, ok := apUsecases.ExtractNote(res.RawJSON)
	if !ok {
		return nil, domainerrors.New(domainerrors.ErrInternal, "created activity did not contain a note")
	}
	note, err := u.cfg.NotesRepo.GetNoteByURI(ctx, nil, extracted.URI)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return note, nil
}

func (u UseCase) HomeTimeline(ctx context.Context, account *models.Account) ([]models.Note, *domainerrors.DomainError) {
	if account == nil {
		return nil, domainerrors.New(domainerrors.ErrUnauthorized, "missing account")
	}
	notes, err := u.cfg.NotesRepo.ListLocalNotes(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return notes, nil
}

func (u UseCase) PublicTimeline(ctx context.Context, account *models.Account) ([]models.Note, *domainerrors.DomainError) {
	return u.HomeTimeline(ctx, account)
}
