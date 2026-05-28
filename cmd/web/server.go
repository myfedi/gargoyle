package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/myfedi/gargoyle/adapters"
	apAdapters "github.com/myfedi/gargoyle/adapters/activitypub"
	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	passwordAdapters "github.com/myfedi/gargoyle/adapters/password"
	"github.com/myfedi/gargoyle/adapters/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
	mastodonUsecases "github.com/myfedi/gargoyle/domain/usecases/mastodon"
	"github.com/myfedi/gargoyle/domain/usecases/oauth"
	infra "github.com/myfedi/gargoyle/infrastructure"
	"github.com/myfedi/gargoyle/infrastructure/db"
	"github.com/myfedi/gargoyle/infrastructure/jobs"
	"github.com/myfedi/gargoyle/infrastructure/web/handlers"
	"github.com/myfedi/gargoyle/infrastructure/web/handlers/mastodon"
	"github.com/myfedi/gargoyle/infrastructure/web/handlers/users"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/myfedi/gargoyle/infrastructure/config"
	"github.com/myfedi/gargoyle/mock"
)

func usersRemoteURLExceptions(exceptions []config.ActivityPubRemoteURLException) []users.RemoteURLException {
	res := make([]users.RemoteURLException, 0, len(exceptions))
	for _, exception := range exceptions {
		res = append(res, users.RemoteURLException{Host: exception.Host, AllowHTTP: exception.AllowHTTP, AllowPrivateIP: exception.AllowPrivateIP})
	}
	return res
}

func mastodonRemoteURLExceptions(exceptions []config.ActivityPubRemoteURLException) []mastodon.RemoteURLException {
	res := make([]mastodon.RemoteURLException, 0, len(exceptions))
	for _, exception := range exceptions {
		res = append(res, mastodon.RemoteURLException{Host: exception.Host, AllowHTTP: exception.AllowHTTP, AllowPrivateIP: exception.AllowPrivateIP})
	}
	return res
}

func main() {
	/// config
	argsWithoutProg := os.Args[1:]
	if len(argsWithoutProg) == 0 {
		panic("config path is required")
	}

	configPath := argsWithoutProg[0]

	config, err := config.NewConfig(configPath)
	if err != nil {
		panic(err)
	}

	// building the host for discovery endpoints
	host := config.Host()

	/// set up adapters and other dependencies
	sqlite := db.NewSqliteStore(db.SqliteStoreConfig{
		Debug:      config.Debug,
		SqlitePath: config.Sqlite.Uri,
	})

	usersRepo := repos.NewUsersRepo(sqlite.Bun)
	accountsRepo := repos.NewAccountsRepo(sqlite.Bun)
	activitiesRepo := repos.NewActivitiesRepo(sqlite.Bun)
	followsRepo := repos.NewFollowsRepo(sqlite.Bun)
	notesRepo := repos.NewNotesRepo(sqlite.Bun)
	remoteAccountsRepo := repos.NewRemoteAccountsRepo(sqlite.Bun)
	oauthRepo := repos.NewOAuthRepo(sqlite.Bun)
	jobsRepo := repos.NewJobsRepo(sqlite.Bun)
	txProvider := dbAdapters.NewBunTxProvider(sqlite.Bun)

	// sets up the go-fiber server. The body limit protects ActivityPub endpoints
	// from unbounded in-memory request bodies before handlers copy or parse them.
	app := fiber.New(fiber.Config{BodyLimit: config.ActivityPub.BodyLimitBytes})
	if len(config.Web.CORS.AllowedOrigins) > 0 {
		app.Use(cors.New(cors.Config{
			AllowOrigins:     strings.Join(config.Web.CORS.AllowedOrigins, ","),
			AllowMethods:     strings.Join(config.Web.CORS.AllowedMethods, ","),
			AllowHeaders:     strings.Join(config.Web.CORS.AllowedHeaders, ","),
			AllowCredentials: config.Web.CORS.AllowCredentials,
		}))
	}

	/// set up the routes

	// set up webfinger handler and dependencies
	webfingerHandler := handlers.NewWebfingerWebHandler(handlers.WebfingerWebHandlerConfig{
		Domain:    config.Domain,
		Host:      host,
		UsersRepo: usersRepo,
	})
	webfingerHandler.SetupWebfinger(app)

	// set up hostmeta handler and dependencies
	hostMetaHandler := handlers.NewHostMetaWebHandler(handlers.HostMetaWebHandlerConfig{
		Host: host,
	})
	hostMetaHandler.SetupHostMeta(app)

	// set up nodeinfo handler and dependencies
	nodeInfoHandler := handlers.NewNodeInfoWebHandler(handlers.NodeInfoHandlerConfig{
		UsersRepo:     usersRepo,
		PostsRepo:     notesRepo,
		CommentsRepo:  &mock.MockCommentsRepository{},
		Host:          host,
		ServerVersion: infra.ServerVersion,
	})
	nodeInfoHandler.SetupNodeInfo(app)

	// set up userprofile handler
	actorSerializer := apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{})
	contentSanitizer := adapters.NewContentSanitizer()
	userRemoteURLExceptions := usersRemoteURLExceptions(config.ActivityPub.RemoteURLExceptions)
	mastodonRemoteURLExceptions := mastodonRemoteURLExceptions(config.ActivityPub.RemoteURLExceptions)
	userProfileHandler := users.NewUsersWebHandler(users.UsersWebHandlerConfig{
		TxProvider:          txProvider,
		AccountsRepo:        accountsRepo,
		ActivitiesRepo:      activitiesRepo,
		FollowsRepo:         followsRepo,
		NotesRepo:           notesRepo,
		DeliveryJobsRepo:    jobsRepo,
		Serializer:          actorSerializer,
		ContentSanitizer:    contentSanitizer,
		BodyLimitBytes:      config.ActivityPub.BodyLimitBytes,
		RemoteURLExceptions: userRemoteURLExceptions,
		DeliveryQueueSize:   config.ActivityPub.DeliveryQueueSize,
		RequireSignedInbox:  true,
		DeliveryRetries:     3,
	})
	userProfileHandler.SetupUserProfileHandler(app)

	// set up Mastodon-compatible OAuth/client API foundation.
	oauthUC := oauth.NewUseCase(oauth.Config{
		OAuthRepo:    oauthRepo,
		UsersRepo:    usersRepo,
		AccountsRepo: accountsRepo,
		PasswordHash: passwordAdapters.NewBCryptPasswordHasher(),
	})
	mastodon.NewOAuthHandler(oauthUC).Setup(app)
	mastodonFlowCfg := apUsecases.ActivityPubFlowConfig{
		TxProvider:       txProvider,
		AccountsRepo:     accountsRepo,
		ActivitiesRepo:   activitiesRepo,
		FollowsRepo:      followsRepo,
		NotesRepo:        notesRepo,
		FetchJobsRepo:    jobsRepo,
		ContentSanitizer: contentSanitizer,
	}
	mastodonAPIUC := mastodonUsecases.NewUseCase(mastodonUsecases.Config{
		Host:               host,
		Domain:             config.Domain,
		ServerVersion:      infra.ServerVersion,
		NotesRepo:          notesRepo,
		FollowsRepo:        followsRepo,
		RemoteAccountsRepo: remoteAccountsRepo,
		IDGenerator:        adapters.NewULIDGenerator(),
		RemoteResolver:     mastodon.NewRemoteAccountResolver(nil, mastodonRemoteURLExceptions),
		CreateOutboxUC:     apUsecases.NewCreateOutboxActivityUseCase(mastodonFlowCfg),
		CreateFollowingUC:  apUsecases.NewCreateFollowingUseCase(mastodonFlowCfg),
	})
	mastodon.NewAPIHandler(mastodon.APIHandlerConfig{OAuth: oauthUC, API: mastodonAPIUC, QueueDelivery: userProfileHandler.QueueDelivery}).Setup(app)

	workerCtx := context.Background()
	jobs.NewDeliveryWorker(jobs.DeliveryWorkerConfig{JobsRepo: jobsRepo, Accounts: accountsRepo, Deliverer: userProfileHandler.ActivityDeliverer()}).Start(workerCtx)
	hydrateRemoteObjectUC := apUsecases.NewHydrateRemoteObjectUseCase(apUsecases.HydrateRemoteObjectConfig{Fetcher: mastodon.NewRemoteObjectFetcher(nil, mastodonRemoteURLExceptions), ActivitiesRepo: activitiesRepo, NotesRepo: notesRepo, Sanitizer: contentSanitizer})
	jobs.NewFetchWorker(jobs.FetchWorkerConfig{JobsRepo: jobsRepo, Accounts: accountsRepo, Hydrater: hydrateRemoteObjectUC}).Start(workerCtx)

	/// run server

	err = app.Listen(fmt.Sprintf(":%d", config.Port))
	if err != nil {
		panic(err)
	}
}
