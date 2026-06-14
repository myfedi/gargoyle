package activitypub

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports"
	apPorts "github.com/myfedi/gargoyle/domain/ports/activitypub"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// PaginationInput describes offset pagination requested by collection endpoints.
type PaginationInput struct {
	Limit  int
	Offset int
}

// ActivityPubFlowConfig wires the ports required by ActivityPub collection and
// mutation use cases. Mutating use cases use TxProvider for atomic local state
// changes; network delivery remains outside the transaction via returned results.
type ActivityPubFlowConfig struct {
	TxProvider         db.TxProvider
	AccountsRepo       repos.AccountsRepo
	ActivitiesRepo     repos.ActivitiesRepository
	FollowsRepo        repos.FollowsRepository
	NotesRepo          repos.NotesRepository
	RemoteAccountsRepo repos.RemoteAccountsRepository
	DomainBlocksRepo   repos.DomainBlocksRepository
	FetchJobsRepo      repos.FetchJobsRepository
	SocialRepo         repos.SocialRepository
	BoostsRepo         repos.BoostsRepository
	PollsRepo          repos.PollsRepository
	MediaRepo          repos.MediaRepository
	MentionsRepo       repos.MentionsRepository
	ActorFetcher       apPorts.ActorFetcher
	ActorSerializer    apPorts.ActorSerializer
	ContentSanitizer   ports.ContentSanitizer
	Host               string
}

// GetOutboxUseCase reads a local actor's outbox collection.
type GetOutboxUseCase struct{ cfg ActivityPubFlowConfig }

// GetFollowersUseCase reads a local actor's accepted followers collection.
type GetFollowersUseCase struct{ cfg ActivityPubFlowConfig }

// GetFollowingUseCase reads the remote actors followed by a local actor.
type GetFollowingUseCase struct{ cfg ActivityPubFlowConfig }

// GetFeaturedUseCase reads a local actor's featured/pinned notes collection.
type GetFeaturedUseCase struct{ cfg ActivityPubFlowConfig }

// GetDereferenceUseCase reads public local ActivityPub objects and activities by their canonical IDs.
type GetDereferenceUseCase struct{ cfg ActivityPubFlowConfig }

// CreateFollowingUseCase creates a local Follow activity and following record atomically.
type CreateFollowingUseCase struct{ cfg ActivityPubFlowConfig }

// UndoFollowingUseCase creates a local Undo Follow activity and removes the following record atomically.
type UndoFollowingUseCase struct{ cfg ActivityPubFlowConfig }

// CreateOutboxActivityUseCase normalizes and stores a local outbox activity atomically.
type CreateOutboxActivityUseCase struct{ cfg ActivityPubFlowConfig }

// CreateInteractionUseCase creates local Like/Announce activities and derived local state atomically.
type CreateInteractionUseCase struct{ cfg ActivityPubFlowConfig }

// UndoInteractionUseCase creates local Undo Like/Announce activities and removes derived local state atomically.
type UndoInteractionUseCase struct{ cfg ActivityPubFlowConfig }

// DeleteObjectUseCase creates a local Delete activity and removes the local object atomically.
type DeleteObjectUseCase struct{ cfg ActivityPubFlowConfig }

// UpdateObjectUseCase creates a local Update activity and updates derived local object state atomically.
type UpdateObjectUseCase struct{ cfg ActivityPubFlowConfig }

// UpdateActorUseCase creates a local Update activity and updates local actor profile state atomically.
type UpdateActorUseCase struct{ cfg ActivityPubFlowConfig }

// FollowRequestDecisionUseCase accepts or rejects pending inbound Follow activities.
type FollowRequestDecisionUseCase struct{ cfg ActivityPubFlowConfig }

// VotePollUseCase stores local votes on ActivityPub Questions and prepares vote deliveries.
type VotePollUseCase struct{ cfg ActivityPubFlowConfig }

// HandleInboxActivityUseCase stores an inbound activity and applies its derived local state atomically.
type HandleInboxActivityUseCase struct{ cfg ActivityPubFlowConfig }

func NewGetOutboxUseCase(cfg ActivityPubFlowConfig) GetOutboxUseCase {
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	return GetOutboxUseCase{cfg: cfg}
}
func NewGetFollowersUseCase(cfg ActivityPubFlowConfig) GetFollowersUseCase {
	requireAccountsRepo(cfg)
	requireFollowsRepo(cfg)
	return GetFollowersUseCase{cfg: cfg}
}
func NewGetFollowingUseCase(cfg ActivityPubFlowConfig) GetFollowingUseCase {
	requireAccountsRepo(cfg)
	requireFollowsRepo(cfg)
	return GetFollowingUseCase{cfg: cfg}
}
func NewGetFeaturedUseCase(cfg ActivityPubFlowConfig) GetFeaturedUseCase {
	requireAccountsRepo(cfg)
	requireNotesRepo(cfg)
	requireSocialRepo(cfg)
	return GetFeaturedUseCase{cfg: cfg}
}
func NewGetDereferenceUseCase(cfg ActivityPubFlowConfig) GetDereferenceUseCase {
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	requireNotesRepo(cfg)
	return GetDereferenceUseCase{cfg: cfg}
}
func NewCreateFollowingUseCase(cfg ActivityPubFlowConfig) CreateFollowingUseCase {
	requireTxProvider(cfg)
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	requireFollowsRepo(cfg)
	return CreateFollowingUseCase{cfg: cfg}
}
func NewUndoFollowingUseCase(cfg ActivityPubFlowConfig) UndoFollowingUseCase {
	requireTxProvider(cfg)
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	requireFollowsRepo(cfg)
	return UndoFollowingUseCase{cfg: cfg}
}
func NewCreateOutboxActivityUseCase(cfg ActivityPubFlowConfig) CreateOutboxActivityUseCase {
	requireTxProvider(cfg)
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	requireFollowsRepo(cfg)
	requireContentSanitizer(cfg)
	return CreateOutboxActivityUseCase{cfg: cfg}
}
func NewCreateInteractionUseCase(cfg ActivityPubFlowConfig) CreateInteractionUseCase {
	requireTxProvider(cfg)
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	requireSocialRepo(cfg)
	return CreateInteractionUseCase{cfg: cfg}
}
func NewUndoInteractionUseCase(cfg ActivityPubFlowConfig) UndoInteractionUseCase {
	requireTxProvider(cfg)
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	requireSocialRepo(cfg)
	return UndoInteractionUseCase{cfg: cfg}
}
func NewDeleteObjectUseCase(cfg ActivityPubFlowConfig) DeleteObjectUseCase {
	requireTxProvider(cfg)
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	requireFollowsRepo(cfg)
	requireNotesRepo(cfg)
	requireMediaRepo(cfg)
	return DeleteObjectUseCase{cfg: cfg}
}
func NewUpdateObjectUseCase(cfg ActivityPubFlowConfig) UpdateObjectUseCase {
	requireTxProvider(cfg)
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	requireFollowsRepo(cfg)
	requireNotesRepo(cfg)
	requireMediaRepo(cfg)
	requirePollsRepo(cfg)
	requireMentionsRepo(cfg)
	return UpdateObjectUseCase{cfg: cfg}
}
func NewUpdateActorUseCase(cfg ActivityPubFlowConfig) UpdateActorUseCase {
	requireTxProvider(cfg)
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	requireFollowsRepo(cfg)
	requireActorSerializer(cfg)
	return UpdateActorUseCase{cfg: cfg}
}
func NewFollowRequestDecisionUseCase(cfg ActivityPubFlowConfig) FollowRequestDecisionUseCase {
	requireTxProvider(cfg)
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	requireFollowsRepo(cfg)
	requireSocialRepo(cfg)
	return FollowRequestDecisionUseCase{cfg: cfg}
}
func NewVotePollUseCase(cfg ActivityPubFlowConfig) VotePollUseCase {
	requireTxProvider(cfg)
	requireAccountsRepo(cfg)
	requireNotesRepo(cfg)
	requirePollsRepo(cfg)
	return VotePollUseCase{cfg: cfg}
}
func NewHandleInboxActivityUseCase(cfg ActivityPubFlowConfig) HandleInboxActivityUseCase {
	requireTxProvider(cfg)
	requireAccountsRepo(cfg)
	requireActivitiesRepo(cfg)
	requireFollowsRepo(cfg)
	requireContentSanitizer(cfg)
	requireHost(cfg)
	return HandleInboxActivityUseCase{cfg: cfg}
}

func requireTxProvider(cfg ActivityPubFlowConfig) {
	if cfg.TxProvider == nil {
		panic("activitypub use case requires TxProvider")
	}
}

func requireAccountsRepo(cfg ActivityPubFlowConfig) {
	if cfg.AccountsRepo == nil {
		panic("activitypub use case requires AccountsRepo")
	}
}

func requireActivitiesRepo(cfg ActivityPubFlowConfig) {
	if cfg.ActivitiesRepo == nil {
		panic("activitypub use case requires ActivitiesRepo")
	}
}

func requireFollowsRepo(cfg ActivityPubFlowConfig) {
	if cfg.FollowsRepo == nil {
		panic("activitypub use case requires FollowsRepo")
	}
}

func requireContentSanitizer(cfg ActivityPubFlowConfig) {
	if cfg.ContentSanitizer == nil {
		panic("activitypub use case requires ContentSanitizer")
	}
}

func requireNotesRepo(cfg ActivityPubFlowConfig) {
	if cfg.NotesRepo == nil {
		panic("activitypub use case requires NotesRepo")
	}
}

func requireMediaRepo(cfg ActivityPubFlowConfig) {
	if cfg.MediaRepo == nil {
		panic("activitypub use case requires MediaRepo")
	}
}

func requirePollsRepo(cfg ActivityPubFlowConfig) {
	if cfg.PollsRepo == nil {
		panic("activitypub use case requires PollsRepo")
	}
}

func requireMentionsRepo(cfg ActivityPubFlowConfig) {
	if cfg.MentionsRepo == nil {
		panic("activitypub use case requires MentionsRepo")
	}
}

func requireActorSerializer(cfg ActivityPubFlowConfig) {
	if cfg.ActorSerializer == nil {
		panic("activitypub use case requires ActorSerializer")
	}
}

func requireSocialRepo(cfg ActivityPubFlowConfig) {
	if cfg.SocialRepo == nil {
		panic("activitypub use case requires SocialRepo")
	}
}

func requireHost(cfg ActivityPubFlowConfig) {
	if cfg.Host == "" {
		panic("activitypub use case requires Host")
	}
}

// localAccount resolves a local ActivityPub account and maps repository errors to domain errors.
func replyIDs(ctx context.Context, repo repos.NotesRepository, tx *db.Tx, note ExtractedNote) (*string, *string) {
	if repo == nil || note.InReplyToURI == nil || *note.InReplyToURI == "" {
		return nil, note.InReplyToURI
	}
	parent, err := repo.GetNoteByURI(ctx, tx, *note.InReplyToURI)
	if err != nil {
		return nil, note.InReplyToURI
	}
	return &parent.ID, note.InReplyToURI
}

func enqueueMissingReplyFetch(ctx context.Context, repo repos.FetchJobsRepository, tx *db.Tx, accountID string, note ExtractedNote, replyID *string) error {
	if repo == nil || note.InReplyToURI == nil || *note.InReplyToURI == "" || replyID != nil {
		return nil
	}
	_, err := repo.CreateFetchJob(ctx, tx, repos.CreateFetchJobInput{URL: *note.InReplyToURI, Kind: "activitypub_object", AccountID: accountID, NextAttemptAt: time.Now().UTC()})
	return err
}

func localAccount(ctx context.Context, repo repos.AccountsRepo, username string) (*models.Account, *domainerrors.DomainError) {
	if username == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "missing username")
	}
	account, err := repo.GetLocalAccountByUsername(ctx, nil, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainerrors.New(domainerrors.ErrNotFound, "no such username")
		}
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return account, nil
}
