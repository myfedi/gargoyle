package main

import (
	"fmt"
	"os"

	apAdapters "github.com/myfedi/gargoyle/adapters/activitypub"
	"github.com/myfedi/gargoyle/adapters/repos"
	infra "github.com/myfedi/gargoyle/infrastructure"
	"github.com/myfedi/gargoyle/infrastructure/db"
	"github.com/myfedi/gargoyle/infrastructure/web/handlers"
	"github.com/myfedi/gargoyle/infrastructure/web/handlers/users"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/infrastructure/config"
	"github.com/myfedi/gargoyle/mock"
)

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

	// sets up the go-fiber server
	app := fiber.New()

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
	userProfileHandler := users.NewUsersWebHandler(users.UsersWebHandlerConfig{
		AccountsRepo:       accountsRepo,
		ActivitiesRepo:     activitiesRepo,
		FollowsRepo:        followsRepo,
		NotesRepo:          notesRepo,
		Serializer:         actorSerializer,
		RequireSignedInbox: true,
		DeliveryRetries:    3,
	})
	userProfileHandler.SetupUserProfileHandler(app)

	/// run server
	err = app.Listen(fmt.Sprintf(":%d", config.Port))
	if err != nil {
		panic(err)
	}
}
