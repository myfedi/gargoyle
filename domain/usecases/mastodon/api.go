package mastodon

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

// RemoteAccountResolver discovers remote ActivityPub accounts for client API
// search/follow workflows without binding domain code to WebFinger or HTTP.
type RemoteAccountResolver interface {
	ResolveAccount(ctx context.Context, query string, signer *models.Account) (*models.Account, error)
}

// Config wires Mastodon-compatible client API workflows to repositories,
// application ports, and lower-level ActivityPub use cases.
type Config struct {
	Host               string
	Domain             string
	ServerVersion      string
	NotesRepo          repos.NotesRepository
	FollowsRepo        repos.FollowsRepository
	RemoteAccountsRepo repos.RemoteAccountsRepository
	IDGenerator        ports.IDGenerator
	RemoteResolver     RemoteAccountResolver
	CreateOutboxUC     apUsecases.CreateOutboxActivityUseCase
	CreateFollowingUC  apUsecases.CreateFollowingUseCase
}

// UseCase groups the Mastodon-compatible client API workflows. Individual
// workflow methods live in focused files so the package can grow without a
// single monolithic implementation file.
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
	if cfg.RemoteAccountsRepo == nil {
		panic("mastodon API use case requires RemoteAccountsRepo")
	}
	if cfg.IDGenerator == nil {
		panic("mastodon API use case requires IDGenerator")
	}
	if cfg.RemoteResolver == nil {
		panic("mastodon API use case requires RemoteResolver")
	}
	return UseCase{cfg: cfg}
}

func requireAccount(account *models.Account) *domainerrors.DomainError {
	if account == nil {
		return domainerrors.New(domainerrors.ErrUnauthorized, "missing account")
	}
	return nil
}

func AccountIDForRemoteActor(actor string) string {
	return "remote:" + base64.RawURLEncoding.EncodeToString([]byte(actor))
}

func RemoteActorFromAccountID(id string) (string, error) {
	unescaped, err := url.PathUnescape(id)
	if err != nil {
		return "", err
	}
	id = unescaped
	if !strings.HasPrefix(id, "remote:") {
		return id, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "remote:"))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
