package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/myfedi/gargoyle/infrastructure/config"
	"github.com/myfedi/gargoyle/infrastructure/server"
)

func main() {
	argsWithoutProg := os.Args[1:]
	if len(argsWithoutProg) == 0 {
		panic("config path is required")
	}

	cfg, err := config.NewConfig(argsWithoutProg[0])
	if err != nil {
		panic(err)
	}

	deps := server.BuildDeps(cfg)
	defer deps.Store.Bun.Close()

	app := server.NewFiberApp(cfg)
	server.MountDiscovery(app, deps.Discovery)
	server.MountActivityPub(app, deps.ActivityPub)
	if cfg.ClientAPI.Enabled {
		server.MountClientAPI(app, deps.ClientAPI)
	}

	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	server.StartCoreWorkers(workerCtx, deps.Workers)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Listen(app, cfg.Port)
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdown)
	select {
	case err := <-serverErr:
		if err != nil {
			panic(err)
		}
	case <-shutdown:
		stopWorkers()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(ctx); err != nil {
			panic(err)
		}
	}
}
