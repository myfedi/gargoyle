package mastodon

import (
	"context"
	"encoding/base64"
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
// RemoteAccountResolver discovers remote ActivityPub accounts for client API
// search without binding the domain workflow to WebFinger or HTTP details.
type RemoteAccountResolver interface {
	ResolveAccount(ctx context.Context, query string, signer *models.Account) (*models.Account, error)
}

type Config struct {
	Host              string
	Domain            string
	ServerVersion     string
	NotesRepo         repos.NotesRepository
	FollowsRepo       repos.FollowsRepository
	IDGenerator       ports.IDGenerator
	RemoteResolver    RemoteAccountResolver
	CreateOutboxUC    apUsecases.CreateOutboxActivityUseCase
	CreateFollowingUC apUsecases.CreateFollowingUseCase
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

type CreateStatusResult struct {
	Note            models.Note
	Account         models.Account
	RawJSON         []byte
	FollowerInboxes []string
}

type FollowAccountResult struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

func NewUseCase(cfg Config) UseCase {
	if cfg.Host == "" || cfg.Domain == "" {
		panic("mastodon API use case requires Host and Domain")
	}
	if cfg.NotesRepo == nil {
		panic("mastodon API use case requires NotesRepo")
	}
	if cfg.FollowsRepo == nil {
		panic("mastodon API use case requires FollowsRepo")
	}
	if cfg.IDGenerator == nil {
		panic("mastodon API use case requires IDGenerator")
	}
	if cfg.RemoteResolver == nil {
		panic("mastodon API use case requires RemoteResolver")
	}
	return UseCase{cfg: cfg}
}

func (u UseCase) InstanceInfo() InstanceInfo {
	return InstanceInfo{Host: u.cfg.Host, Domain: u.cfg.Domain, Title: "Gargoyle", Description: "Gargoyle federated server", ServerVersion: u.cfg.ServerVersion}
}

func (u UseCase) CreateStatus(ctx context.Context, account *models.Account, content string) (*CreateStatusResult, *domainerrors.DomainError) {
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
	return &CreateStatusResult{Note: *note, Account: res.Account, RawJSON: res.RawJSON, FollowerInboxes: res.FollowerInboxes}, nil
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

func (u UseCase) SearchAccounts(ctx context.Context, account *models.Account, query string) ([]models.Account, *domainerrors.DomainError) {
	if account == nil {
		return nil, domainerrors.New(domainerrors.ErrUnauthorized, "missing account")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []models.Account{}, nil
	}
	remote, err := u.cfg.RemoteResolver.ResolveAccount(ctx, query, account)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	return []models.Account{*remote}, nil
}

func (u UseCase) FollowAccount(ctx context.Context, localAccount *models.Account, accountID string) (*FollowAccountResult, *domainerrors.DomainError) {
	if localAccount == nil {
		return nil, domainerrors.New(domainerrors.ErrUnauthorized, "missing account")
	}
	actor, err := RemoteActorFromAccountID(accountID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	remote, err := u.cfg.RemoteResolver.ResolveAccount(ctx, actor, localAccount)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	followID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res, derr := u.cfg.CreateFollowingUC.CreateFollowing(ctx, apUsecases.CreateFollowingInput{Username: localAccount.Username, Actor: remote.URI, Inbox: remote.InboxURI, FollowID: followID})
	if derr != nil {
		return nil, derr
	}
	return &FollowAccountResult{Account: res.Account, RawJSON: res.RawJSON, Inbox: res.Inbox}, nil
}

func (u UseCase) Relationships(ctx context.Context, localAccount *models.Account, ids []string) (map[string]bool, *domainerrors.DomainError) {
	if localAccount == nil {
		return nil, domainerrors.New(domainerrors.ErrUnauthorized, "missing account")
	}
	following, err := u.cfg.FollowsRepo.ListFollowing(ctx, nil, localAccount.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	byActor := map[string]bool{}
	for _, follow := range following {
		byActor[follow.RemoteActor] = true
	}
	res := map[string]bool{}
	for _, id := range ids {
		actor, err := RemoteActorFromAccountID(id)
		if err != nil {
			res[id] = false
			continue
		}
		res[id] = byActor[actor]
	}
	return res, nil
}

func AccountIDForRemoteActor(actor string) string {
	return "remote:" + base64.RawURLEncoding.EncodeToString([]byte(actor))
}

func RemoteActorFromAccountID(id string) (string, error) {
	if !strings.HasPrefix(id, "remote:") {
		return id, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "remote:"))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
