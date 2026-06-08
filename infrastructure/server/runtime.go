package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/myfedi/gargoyle/adapters"
	apAdapters "github.com/myfedi/gargoyle/adapters/activitypub"
	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	passwordAdapters "github.com/myfedi/gargoyle/adapters/password"
	"github.com/myfedi/gargoyle/adapters/repos"
	"github.com/myfedi/gargoyle/domain/ports"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
	clientapiUsecases "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/domain/usecases/oauth"
	infra "github.com/myfedi/gargoyle/infrastructure"
	"github.com/myfedi/gargoyle/infrastructure/config"
	"github.com/myfedi/gargoyle/infrastructure/db"
	"github.com/myfedi/gargoyle/infrastructure/jobs"
	"github.com/myfedi/gargoyle/infrastructure/web/handlers"
	activitypubHandlers "github.com/myfedi/gargoyle/infrastructure/web/handlers/activitypub"
	clientapiHandlers "github.com/myfedi/gargoyle/infrastructure/web/handlers/clientapi"
)

// Deps contains the wired runtime dependencies shared by the ActivityPub core,
// optional client API surface, and background workers.
type Deps struct {
	Store db.SqliteStore

	Discovery   DiscoveryDeps
	ActivityPub ActivityPubDeps
	ClientAPI   ClientAPIDeps
	Workers     WorkerDeps
}

type DiscoveryDeps struct {
	Webfinger *handlers.WebfingerWebHandler
	HostMeta  *handlers.HostMetaWebHandler
	NodeInfo  *handlers.NodeInfoWebHandler
}

type ActivityPubDeps struct {
	Handler *activitypubHandlers.Handler
}

type ClientAPIDeps struct {
	OAuth              oauth.UseCase
	Instance           clientapiUsecases.Instance
	Accounts           clientapiUsecases.Accounts
	Statuses           clientapiUsecases.Statuses
	Timelines          clientapiUsecases.Timelines
	Interactions       clientapiUsecases.Interactions
	Notifications      clientapiUsecases.Notifications
	Conversations      clientapiUsecases.Conversations
	Media              clientapiUsecases.Media
	Profile            clientapiUsecases.Profile
	Moderation         clientapiUsecases.Moderation
	ActivityPubHandler *activitypubHandlers.Handler
}

type WorkerDeps struct {
	JobsRepo             *repos.JobsRepo
	AccountsRepo         *repos.AccountsRepo
	ModerationRepo       *repos.ModerationRepo
	MediaRepo            *repos.MediaRepo
	BoostsRepo           *repos.BoostsRepo
	RemoteAccountsRepo   *repos.RemoteAccountsRepo
	MediaStorage         *adapters.LocalMediaStorage
	MediaCleanupInterval time.Duration
	MediaUnattachedTTL   time.Duration
	RemoteCacheMaxBytes  int64
	RemoteCacheTTL       time.Duration
	RemoteObjectFetcher  clientapiHandlers.RemoteObjectFetcher
	RemoteMediaFetcher   clientapiHandlers.RemoteMediaFetcher
	ActivitiesRepo       *repos.ActivitiesRepo
	NotesRepo            *repos.NotesRepo
	ContentSanitizer     ports.ContentSanitizer
	Moderation           clientapiUsecases.Moderation
	ActivityPubHandler   *activitypubHandlers.Handler
}

func activityPubRemoteURLExceptions(exceptions []config.ActivityPubRemoteURLException) []activitypubHandlers.RemoteURLException {
	res := make([]activitypubHandlers.RemoteURLException, 0, len(exceptions))
	for _, exception := range exceptions {
		res = append(res, activitypubHandlers.RemoteURLException{Host: exception.Host, AllowHTTP: exception.AllowHTTP, AllowPrivateIP: exception.AllowPrivateIP})
	}
	return res
}

func clientAPIRemoteURLExceptions(exceptions []config.ActivityPubRemoteURLException) []clientapiHandlers.RemoteURLException {
	res := make([]clientapiHandlers.RemoteURLException, 0, len(exceptions))
	for _, exception := range exceptions {
		res = append(res, clientapiHandlers.RemoteURLException{Host: exception.Host, AllowHTTP: exception.AllowHTTP, AllowPrivateIP: exception.AllowPrivateIP})
	}
	return res
}

func NewFiberApp(cfg *config.Config) *fiber.App {
	bodyLimitBytes := cfg.ActivityPub.BodyLimitBytes
	if mediaLimit := clientapiUsecases.MaxMediaUploadBytes + (1 << 20); bodyLimitBytes < mediaLimit {
		bodyLimitBytes = mediaLimit
	}
	app := fiber.New(fiber.Config{BodyLimit: bodyLimitBytes})
	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{Format: "${time} request_id=${locals:requestid} method=${method} path=${path} status=${status} latency=${latency} ip=${ip} ua=\"${ua}\" error=\"${error}\"\n"}))
	app.Use(limiter.New(limiter.Config{Max: 300, Expiration: 1 * time.Minute}))
	app.Use("/oauth", limiter.New(limiter.Config{Max: 20, Expiration: 1 * time.Minute}))
	app.Use("/api/v1/apps", limiter.New(limiter.Config{Max: 10, Expiration: 1 * time.Hour}))
	app.Use("/api/v1/media", limiter.New(limiter.Config{Max: 30, Expiration: 1 * time.Minute}))
	app.Use("/api/v2/media", limiter.New(limiter.Config{Max: 30, Expiration: 1 * time.Minute}))
	app.Use("/users/:username/inbox", limiter.New(limiter.Config{Max: 120, Expiration: 1 * time.Minute}))
	if len(cfg.Web.CORS.AllowedOrigins) > 0 {
		app.Use(cors.New(cors.Config{
			AllowOrigins:     strings.Join(cfg.Web.CORS.AllowedOrigins, ","),
			AllowMethods:     strings.Join(cfg.Web.CORS.AllowedMethods, ","),
			AllowHeaders:     strings.Join(cfg.Web.CORS.AllowedHeaders, ","),
			AllowCredentials: cfg.Web.CORS.AllowCredentials,
		}))
	}
	return app
}

func BuildDeps(cfg *config.Config) *Deps {
	host := cfg.Host()

	sqlite := db.NewSqliteStore(db.SqliteStoreConfig{Debug: cfg.Debug, SqlitePath: cfg.Sqlite.Uri})

	usersRepo := repos.NewUsersRepo(sqlite.Bun)
	accountsRepo := repos.NewAccountsRepo(sqlite.Bun)
	activitiesRepo := repos.NewActivitiesRepo(sqlite.Bun)
	followsRepo := repos.NewFollowsRepo(sqlite.Bun)
	notesRepo := repos.NewNotesRepo(sqlite.Bun)
	mediaRepo := repos.NewMediaRepo(sqlite.Bun)
	mediaStorage := adapters.NewLocalMediaStorage(cfg.Media.StorageDir)
	socialRepo := repos.NewSocialRepo(sqlite.Bun)
	boostsRepo := repos.NewBoostsRepo(sqlite.Bun)
	pollsRepo := repos.NewPollsRepo(sqlite.Bun)
	conversationsRepo := repos.NewConversationsRepo(sqlite.Bun)
	mentionsRepo := repos.NewMentionsRepo(sqlite.Bun)
	remoteAccountsRepo := repos.NewRemoteAccountsRepo(sqlite.Bun)
	moderationRepo := repos.NewModerationRepo(sqlite.Bun)
	oauthRepo := repos.NewOAuthRepo(sqlite.Bun)
	jobsRepo := repos.NewJobsRepo(sqlite.Bun)
	txProvider := dbAdapters.NewBunTxProvider(sqlite.Bun)

	webfingerHandler := handlers.NewWebfingerWebHandler(handlers.WebfingerWebHandlerConfig{Domain: cfg.Domain, Host: host, UsersRepo: usersRepo})
	hostMetaHandler := handlers.NewHostMetaWebHandler(handlers.HostMetaWebHandlerConfig{Host: host})
	nodeInfoHandler := handlers.NewNodeInfoWebHandler(handlers.NodeInfoHandlerConfig{UsersRepo: usersRepo, PostsRepo: notesRepo, CommentsRepo: notesRepo, Host: host, ServerVersion: infra.ServerVersion})

	actorSerializer := apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{})
	contentSanitizer := adapters.NewContentSanitizer()
	activityPubURLExceptions := activityPubRemoteURLExceptions(cfg.ActivityPub.RemoteURLExceptions)
	clientAPIURLExceptions := clientAPIRemoteURLExceptions(cfg.ActivityPub.RemoteURLExceptions)
	userProfileHandler := activitypubHandlers.NewHandler(activitypubHandlers.HandlerConfig{
		TxProvider:          txProvider,
		AccountsRepo:        accountsRepo,
		ActivitiesRepo:      activitiesRepo,
		FollowsRepo:         followsRepo,
		NotesRepo:           notesRepo,
		SocialRepo:          socialRepo,
		BoostsRepo:          boostsRepo,
		PollsRepo:           pollsRepo,
		RemoteAccountsRepo:  remoteAccountsRepo,
		DomainBlocksRepo:    moderationRepo,
		DeliveryJobsRepo:    jobsRepo,
		FetchJobsRepo:       jobsRepo,
		MediaRepo:           mediaRepo,
		Serializer:          actorSerializer,
		ContentSanitizer:    contentSanitizer,
		BodyLimitBytes:      cfg.ActivityPub.BodyLimitBytes,
		RemoteURLExceptions: activityPubURLExceptions,
		RequireSignedInbox:  true,
		DeliveryRetries:     3,
		Host:                host,
	})

	oauthUC := oauth.NewUseCase(oauth.Config{
		OAuthRepo:          oauthRepo,
		UsersRepo:          usersRepo,
		AccountsRepo:       accountsRepo,
		FollowsRepo:        followsRepo,
		NotesRepo:          notesRepo,
		PasswordHash:       passwordAdapters.NewBCryptPasswordHasher(),
		TxProvider:         txProvider,
		AllowPasswordGrant: cfg.OAuth.AllowPasswordGrant,
	})
	clientAPIFlowCfg := apUsecases.ActivityPubFlowConfig{
		TxProvider:         txProvider,
		AccountsRepo:       accountsRepo,
		ActivitiesRepo:     activitiesRepo,
		FollowsRepo:        followsRepo,
		NotesRepo:          notesRepo,
		RemoteAccountsRepo: remoteAccountsRepo,
		DomainBlocksRepo:   moderationRepo,
		FetchJobsRepo:      jobsRepo,
		SocialRepo:         socialRepo,
		BoostsRepo:         boostsRepo,
		PollsRepo:          pollsRepo,
		MediaRepo:          mediaRepo,
		MentionsRepo:       mentionsRepo,
		ActorSerializer:    actorSerializer,
		ContentSanitizer:   contentSanitizer,
		Host:               host,
	}
	clientAPIComponents := buildClientAPIComponents(clientAPIWorkflowInputsFromRuntime(
		host,
		cfg.Domain,
		txProvider,
		runtimeRepos{
			Accounts:       accountsRepo,
			Activities:     activitiesRepo,
			Notes:          notesRepo,
			Follows:        followsRepo,
			Media:          mediaRepo,
			Social:         socialRepo,
			Boosts:         boostsRepo,
			Polls:          pollsRepo,
			Conversations:  conversationsRepo,
			Mentions:       mentionsRepo,
			RemoteAccounts: remoteAccountsRepo,
			Moderation:     moderationRepo,
		},
		mediaStorage,
		contentSanitizer,
		clientAPIFlowCfg,
		clientAPIURLExceptions,
	))

	return &Deps{
		Store:       sqlite,
		Discovery:   DiscoveryDeps{Webfinger: webfingerHandler, HostMeta: hostMetaHandler, NodeInfo: nodeInfoHandler},
		ActivityPub: ActivityPubDeps{Handler: userProfileHandler},
		ClientAPI:   ClientAPIDeps{OAuth: oauthUC, Instance: clientAPIComponents.Instance, Accounts: clientAPIComponents.Accounts, Statuses: clientAPIComponents.Statuses, Timelines: clientAPIComponents.Timelines, Interactions: clientAPIComponents.Interactions, Notifications: clientAPIComponents.Notifications, Conversations: clientAPIComponents.Conversations, Media: clientAPIComponents.Media, Profile: clientAPIComponents.Profile, Moderation: clientAPIComponents.Moderation, ActivityPubHandler: userProfileHandler},
		Workers: WorkerDeps{
			JobsRepo:             jobsRepo,
			AccountsRepo:         accountsRepo,
			ModerationRepo:       moderationRepo,
			MediaRepo:            mediaRepo,
			BoostsRepo:           boostsRepo,
			RemoteAccountsRepo:   remoteAccountsRepo,
			MediaStorage:         mediaStorage,
			MediaCleanupInterval: cfg.Media.CleanupInterval,
			MediaUnattachedTTL:   cfg.Media.UnattachedTTL,
			RemoteCacheMaxBytes:  cfg.Media.RemoteCacheMaxBytes,
			RemoteCacheTTL:       cfg.Media.RemoteCacheTTL,
			RemoteObjectFetcher:  clientAPIComponents.RemoteObjectFetcher,
			RemoteMediaFetcher:   clientapiHandlers.NewRemoteMediaFetcher(nil, clientAPIURLExceptions),
			ActivitiesRepo:       activitiesRepo,
			NotesRepo:            notesRepo,
			ContentSanitizer:     contentSanitizer,
			Moderation:           clientAPIComponents.Moderation,
			ActivityPubHandler:   userProfileHandler,
		},
	}
}

func MountDiscovery(app *fiber.App, deps DiscoveryDeps) {
	deps.Webfinger.SetupWebfinger(app)
	deps.HostMeta.SetupHostMeta(app)
	deps.NodeInfo.SetupNodeInfo(app)
}

func MountActivityPub(app *fiber.App, deps ActivityPubDeps) {
	deps.Handler.SetupRoutes(app)
}

func MountClientAPI(app *fiber.App, deps ClientAPIDeps) {
	clientapiHandlers.NewOAuthHandler(deps.OAuth).Setup(app)
	clientapiHandlers.NewAPIHandler(clientapiHandlers.APIHandlerConfig{OAuth: deps.OAuth, Instance: deps.Instance, Accounts: deps.Accounts, Statuses: deps.Statuses, Timelines: deps.Timelines, Interactions: deps.Interactions, Notifications: deps.Notifications, Conversations: deps.Conversations, Media: deps.Media, Profile: deps.Profile, Moderation: deps.Moderation, QueueDelivery: deps.ActivityPubHandler.QueueDelivery}).Setup(app)
}

func StartCoreWorkers(ctx context.Context, deps WorkerDeps) {
	jobs.NewDeliveryWorker(jobs.DeliveryWorkerConfig{JobsRepo: deps.JobsRepo, Accounts: deps.AccountsRepo, Deliverer: deps.ActivityPubHandler.ActivityDeliverer(), Blocks: deps.ModerationRepo}).Start(ctx)
	hydrateRemoteObjectUC := apUsecases.NewHydrateRemoteObjectUseCase(apUsecases.HydrateRemoteObjectConfig{Fetcher: deps.RemoteObjectFetcher, ActivitiesRepo: deps.ActivitiesRepo, NotesRepo: deps.NotesRepo, MediaRepo: deps.MediaRepo, MediaStorage: deps.MediaStorage, RemoteMediaFetcher: deps.RemoteMediaFetcher, BoostsRepo: deps.BoostsRepo, RemoteAccountsRepo: deps.RemoteAccountsRepo, Sanitizer: deps.ContentSanitizer})
	jobs.NewFetchWorker(jobs.FetchWorkerConfig{JobsRepo: deps.JobsRepo, Accounts: deps.AccountsRepo, Hydrater: hydrateRemoteObjectUC, Blocks: deps.ModerationRepo}).Start(ctx)
	jobs.NewMediaCleanupWorker(jobs.MediaCleanupWorkerConfig{MediaRepo: deps.MediaRepo, Storage: deps.MediaStorage, Interval: deps.MediaCleanupInterval, UnattachedTTL: deps.MediaUnattachedTTL, RemoteCacheMaxBytes: deps.RemoteCacheMaxBytes, RemoteCacheTTL: deps.RemoteCacheTTL}).Start(ctx)
	jobs.NewModerationWorker(jobs.ModerationWorkerConfig{JobsRepo: deps.ModerationRepo, API: deps.Moderation, MediaStorage: deps.MediaStorage}).Start(ctx)
}

func Listen(app *fiber.App, port int) error {
	return app.Listen(fmt.Sprintf(":%d", port))
}
