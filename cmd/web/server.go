package main

import (
	"fmt"
	"os"

	infra "github.com/myfedi/gargoyle/infrastructure"
	"github.com/myfedi/gargoyle/infrastructure/db"
	"github.com/myfedi/gargoyle/infrastructure/web/handlers"

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
	var host string
	if config.Tls {
		host = fmt.Sprintf("https://%s", config.Domain)
		if config.Port != 443 {
			host += fmt.Sprintf(":%d", config.Port)
		}
	} else {
		host = fmt.Sprintf("http://%s", config.Domain)
		if config.Port != 80 {
			host += fmt.Sprintf(":%d", config.Port)
		}
	}

	/// set up adapters and other dependencies
	_ = db.NewSqliteStore(db.SqliteStoreConfig{
		Debug:      config.Debug,
		SqlitePath: config.Sqlite.Uri,
	})

	// sets up the go-fiber server
	app := fiber.New()

	/// set up the routes

	// set up webfinger handler and dependencies
	webfingerHandler := handlers.NewWebfingerWebHandler(handlers.WebfingerWebHandlerConfig{
		Domain:    config.Domain,
		Host:      host,
		UsersRepo: &mock.MockUsersRepository{},
	})
	webfingerHandler.SetupWebfinger(app)

	// set up hostmeta handler and dependencies
	hostMetaHandler := handlers.NewHostMetaWebHandler(handlers.HostMetaWebHandlerConfig{
		Host: host,
	})
	hostMetaHandler.SetupHostMeta(app)

	// set up nodeinfo handler and dependencies
	nodeInfoHandler := handlers.NewNodeInfoWebHandler(handlers.NodeInfoHandlerConfig{
		// FIXME: replace with actual repo adapters
		UsersRepo:     &mock.MockUsersRepository{},
		PostsRepo:     &mock.MockPostsRepository{},
		CommentsRepo:  &mock.MockCommentsRepository{},
		Host:          host,
		ServerVersion: infra.ServerVersion,
	})
	nodeInfoHandler.SetupNodeInfo(app)

	/// run server
	err = app.Listen(fmt.Sprintf(":%d", config.Port))
	if err != nil {
		panic(err)
	}
}
