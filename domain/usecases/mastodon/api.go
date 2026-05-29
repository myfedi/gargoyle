package mastodon

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"
	"time"

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
	AccountsRepo       repos.AccountsRepo
	NotesRepo          repos.NotesRepository
	FollowsRepo        repos.FollowsRepository
	MediaRepo          repos.MediaRepository
	MediaStorage       ports.MediaStorage
	SocialRepo         repos.SocialRepository
	BoostsRepo         repos.BoostsRepository
	ConversationsRepo  repos.ConversationsRepository
	MentionsRepo       repos.MentionsRepository
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

type CreateStatusInput struct {
	Content     string
	InReplyToID string
	Visibility  string
	Sensitive   bool
	SpoilerText string
	MediaIDs    []string
}

type CreateStatusResult struct {
	Note            models.Note
	Account         models.Account
	Media           []models.MediaAttachment
	Mentions        []models.Mention
	RawJSON         []byte
	FollowerInboxes []string
	MentionInboxes  []string
}

type TimelineItem struct {
	ID                 string
	URI                string
	CreatedAt          time.Time
	Note               models.Note
	Account            models.Account
	InReplyToAccountID *string
	Media              []models.MediaAttachment
	Mentions           []models.Mention
	Reblog             *TimelineItem
	Reblogged          bool
	Favourited         bool
	Bookmarked         bool
	ReblogsCount       int
}

type TimelineOptions struct {
	Limit      int
	MaxID      string
	LocalOnly  bool
	RemoteOnly bool
}

type FollowAccountResult struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

type Relationship struct {
	ID        string
	Following bool
	Requested bool
}

func NewUseCase(cfg Config) UseCase {
	if cfg.Host == "" || cfg.Domain == "" {
		panic("mastodon API use case requires Host and Domain")
	}
	if cfg.AccountsRepo == nil {
		panic("mastodon API use case requires AccountsRepo")
	}
	if cfg.NotesRepo == nil {
		panic("mastodon API use case requires NotesRepo")
	}
	if cfg.FollowsRepo == nil {
		panic("mastodon API use case requires FollowsRepo")
	}
	if cfg.MediaRepo == nil {
		panic("mastodon API use case requires MediaRepo")
	}
	if cfg.MediaStorage == nil {
		panic("mastodon API use case requires MediaStorage")
	}
	if cfg.SocialRepo == nil {
		panic("mastodon API use case requires SocialRepo")
	}
	if cfg.BoostsRepo == nil {
		panic("mastodon API use case requires BoostsRepo")
	}
	if cfg.ConversationsRepo == nil {
		panic("mastodon API use case requires ConversationsRepo")
	}
	if cfg.MentionsRepo == nil {
		panic("mastodon API use case requires MentionsRepo")
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
