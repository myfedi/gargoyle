package clientapi

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

// RemoteAccountResolver discovers remote ActivityPub accounts for client API
// search/follow workflows without binding domain code to WebFinger or HTTP.
type RemoteAccountResolver interface {
	ResolveAccount(ctx context.Context, query string, signer *models.Account) (*models.Account, error)
}

type CommonConfig struct {
	Host          string
	Domain        string
	ServerVersion string
}

type InstanceConfig struct{ CommonConfig }

type AccountsConfig struct {
	CommonConfig
	AccountsRepo          repos.AccountsRepo
	NotesRepo             repos.NotesRepository
	FollowsRepo           repos.FollowsRepository
	MediaRepo             repos.MediaRepository
	MediaStorage          ports.MediaStorage
	RemoteMediaFetcher    ports.RemoteMediaFetcher
	SocialRepo            repos.SocialRepository
	BoostsRepo            repos.BoostsRepository
	MentionsRepo          repos.MentionsRepository
	PollsRepo             repos.PollsRepository
	RemoteAccountsRepo    repos.RemoteAccountsRepository
	DomainBlocksRepo      repos.DomainBlocksRepository
	IDGenerator           ports.IDGenerator
	RemoteResolver        RemoteAccountResolver
	CreateFollowingUC     apUsecases.CreateFollowingUseCase
	UndoFollowingUC       apUsecases.UndoFollowingUseCase
	FollowDecisionUC      apUsecases.FollowRequestDecisionUseCase
	HydrateRemoteObjectUC apUsecases.HydrateRemoteObjectUseCase
}

type StatusesConfig struct {
	CommonConfig
	NotesRepo             repos.NotesRepository
	AccountsRepo          repos.AccountsRepo
	MediaRepo             repos.MediaRepository
	MediaStorage          ports.MediaStorage
	SocialRepo            repos.SocialRepository
	BoostsRepo            repos.BoostsRepository
	MentionsRepo          repos.MentionsRepository
	PollsRepo             repos.PollsRepository
	DomainBlocksRepo      repos.DomainBlocksRepository
	RemoteAccountsRepo    repos.RemoteAccountsRepository
	RemoteResolver        RemoteAccountResolver
	ContentSanitizer      ports.ContentSanitizer
	IDGenerator           ports.IDGenerator
	CreateOutboxUC        apUsecases.CreateOutboxActivityUseCase
	DeleteObjectUC        apUsecases.DeleteObjectUseCase
	UpdateObjectUC        apUsecases.UpdateObjectUseCase
	HydrateRemoteObjectUC apUsecases.HydrateRemoteObjectUseCase
}

type TimelinesConfig struct {
	CommonConfig
	NotesRepo          repos.NotesRepository
	AccountsRepo       repos.AccountsRepo
	FollowsRepo        repos.FollowsRepository
	MediaRepo          repos.MediaRepository
	SocialRepo         repos.SocialRepository
	BoostsRepo         repos.BoostsRepository
	MentionsRepo       repos.MentionsRepository
	PollsRepo          repos.PollsRepository
	RemoteAccountsRepo repos.RemoteAccountsRepository
	DomainBlocksRepo   repos.DomainBlocksRepository
	RemoteResolver     RemoteAccountResolver
}

type InteractionsConfig struct {
	CommonConfig
	NotesRepo           repos.NotesRepository
	AccountsRepo        repos.AccountsRepo
	MediaRepo           repos.MediaRepository
	SocialRepo          repos.SocialRepository
	BoostsRepo          repos.BoostsRepository
	MentionsRepo        repos.MentionsRepository
	PollsRepo           repos.PollsRepository
	RemoteAccountsRepo  repos.RemoteAccountsRepository
	DomainBlocksRepo    repos.DomainBlocksRepository
	RemoteResolver      RemoteAccountResolver
	IDGenerator         ports.IDGenerator
	CreateInteractionUC apUsecases.CreateInteractionUseCase
	UndoInteractionUC   apUsecases.UndoInteractionUseCase
	VotePollUC          apUsecases.VotePollUseCase
}

type NotificationsConfig struct {
	CommonConfig
	AccountsRepo       repos.AccountsRepo
	NotesRepo          repos.NotesRepository
	MediaRepo          repos.MediaRepository
	SocialRepo         repos.SocialRepository
	BoostsRepo         repos.BoostsRepository
	MentionsRepo       repos.MentionsRepository
	PollsRepo          repos.PollsRepository
	RemoteAccountsRepo repos.RemoteAccountsRepository
	DomainBlocksRepo   repos.DomainBlocksRepository
	RemoteResolver     RemoteAccountResolver
}

type ConversationsConfig struct {
	CommonConfig
	AccountsRepo       repos.AccountsRepo
	NotesRepo          repos.NotesRepository
	MediaRepo          repos.MediaRepository
	SocialRepo         repos.SocialRepository
	BoostsRepo         repos.BoostsRepository
	MentionsRepo       repos.MentionsRepository
	PollsRepo          repos.PollsRepository
	ConversationsRepo  repos.ConversationsRepository
	RemoteAccountsRepo repos.RemoteAccountsRepository
	DomainBlocksRepo   repos.DomainBlocksRepository
	RemoteResolver     RemoteAccountResolver
}

type MediaConfig struct {
	MediaRepo          repos.MediaRepository
	MediaStorage       ports.MediaStorage
	RemoteMediaFetcher ports.RemoteMediaFetcher
	IDGenerator        ports.IDGenerator
}

type ProfileConfig struct {
	MediaRepo        repos.MediaRepository
	MediaStorage     ports.MediaStorage
	ContentSanitizer ports.ContentSanitizer
	IDGenerator      ports.IDGenerator
	UpdateActorUC    apUsecases.UpdateActorUseCase
}

type ModerationConfig struct {
	TxProvider         db.TxProvider
	DomainBlocksRepo   repos.DomainBlocksRepository
	ModerationJobsRepo repos.ModerationJobsRepository
	DomainPurgeRepo    repos.DomainPurgeRepository
}
