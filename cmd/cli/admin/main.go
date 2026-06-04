package main

import (
	"context"
	"fmt"
	"log"
	"net/mail"
	"strings"
	"time"

	"os"

	"github.com/myfedi/gargoyle/adapters"
	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	"github.com/myfedi/gargoyle/adapters/gcrypto"
	pw "github.com/myfedi/gargoyle/adapters/password"
	"github.com/myfedi/gargoyle/adapters/repos"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/usecases/users"
	"github.com/myfedi/gargoyle/infrastructure/config"
	"github.com/myfedi/gargoyle/infrastructure/db"
	"github.com/myfedi/gargoyle/utils"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/urfave/cli/v2"
)

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func main() {
	app := &cli.App{
		Name:  "admin",
		Usage: "Admin CLI for user management",
		Commands: []*cli.Command{
			{
				Name:  "media-cleanup",
				Usage: "Delete broken media metadata and unattached uploads older than a duration",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.DurationFlag{Name: "older-than", Value: 24 * time.Hour},
					&cli.IntFlag{Name: "limit", Value: 100},
				},
				Action: func(c *cli.Context) error {
					cfg, err := config.NewConfig(c.String("config"))
					if err != nil {
						return err
					}
					store := db.NewSqliteStore(db.SqliteStoreConfig{Debug: cfg.Debug, SqlitePath: cfg.Sqlite.Uri})
					mediaRepo := repos.NewMediaRepo(store.Bun)
					mediaStorage := adapters.NewLocalMediaStorage(cfg.Media.StorageDir)
					ctx := context.Background()
					limit := c.Int("limit")
					if limit <= 0 || limit > 500 {
						limit = 100
					}
					deletedBroken := 0
					broken, err := mediaRepo.ListMediaWithoutStorage(ctx, nil, limit)
					if err != nil {
						return err
					}
					for _, media := range broken {
						if err := mediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
							return err
						}
						deletedBroken++
					}
					deletedUnattached := 0
					remaining := limit - deletedBroken
					if remaining > 0 {
						unattached, err := mediaRepo.ListUnattachedMediaOlderThan(ctx, nil, time.Now().UTC().Add(-c.Duration("older-than")), remaining)
						if err != nil {
							return err
						}
						for _, media := range unattached {
							if err := mediaRepo.DeleteMediaAttachment(ctx, nil, media.ID); err != nil {
								return err
							}
							if err := mediaStorage.DeleteMedia(ctx, media.StoragePath); err != nil {
								return err
							}
							deletedUnattached++
						}
					}
					fmt.Printf("deleted broken=%d unattached=%d\n", deletedBroken, deletedUnattached)
					return nil
				},
			},
			{
				Name:  "jobs",
				Usage: "Inspect durable delivery/fetch jobs",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "type", Value: "delivery", Usage: "delivery or fetch"},
					&cli.StringFlag{Name: "status", Value: "pending", Usage: "pending, failed, done, running"},
					&cli.IntFlag{Name: "limit", Value: 25},
				},
				Action: func(c *cli.Context) error {
					cfg, err := config.NewConfig(c.String("config"))
					if err != nil {
						return err
					}
					store := db.NewSqliteStore(db.SqliteStoreConfig{Debug: cfg.Debug, SqlitePath: cfg.Sqlite.Uri})
					jobsRepo := repos.NewJobsRepo(store.Bun)
					status := models.JobStatus(c.String("status"))
					ctx := context.Background()
					if c.String("type") == "fetch" {
						jobs, err := jobsRepo.ListFetchJobsByStatus(ctx, nil, status, c.Int("limit"))
						if err != nil {
							return err
						}
						for _, job := range jobs {
							fmt.Printf("%s\t%s\t%s\tattempts=%d\tnext=%s\terr=%s\n", job.ID, job.Status, job.URL, job.Attempts, job.NextAttemptAt.Format(time.RFC3339), stringValue(job.LastError))
						}
						return nil
					}
					jobs, err := jobsRepo.ListDeliveryJobsByStatus(ctx, nil, status, c.Int("limit"))
					if err != nil {
						return err
					}
					for _, job := range jobs {
						fmt.Printf("%s\t%s\t%s\tattempts=%d\tnext=%s\terr=%s\n", job.ID, job.Status, job.InboxURL, job.Attempts, job.NextAttemptAt.Format(time.RFC3339), stringValue(job.LastError))
					}
					return nil
				},
			},
			{
				Name:  "register",
				Usage: "Register a new user",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "email", Required: true},
					&cli.StringFlag{Name: "username", Required: true},
					&cli.StringFlag{Name: "password", Required: true},
					&cli.StringFlag{
						Name:     "config",
						Usage:    "Path to config file",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					config, err := config.NewConfig(c.String("config"))
					if err != nil {
						panic(err)
					}

					/// set up adapters and other dependencies
					store := db.NewSqliteStore(db.SqliteStoreConfig{
						Debug:      config.Debug,
						SqlitePath: config.Sqlite.Uri,
					})

					db := bun.NewDB(store.Bun.DB, sqlitedialect.New())
					db.AddQueryHook(bundebug.NewQueryHook(
						bundebug.WithEnabled(false),
						bundebug.FromEnv(),
					))

					// verify and prepare input
					email := strings.TrimSpace(strings.ToLower(c.String("email")))
					username := strings.TrimSpace(c.String("username"))
					password := c.String("password")

					// Email validation
					if _, err := mail.ParseAddress(email); err != nil {
						return fmt.Errorf("invalid email format")
					}

					// Password validation
					if len(password) < 12 {
						return fmt.Errorf("password must be at least 12 characters long")
					}

					// Username validation
					validUsername, err := utils.ValidateAndNormalizeFediUsername(username)
					if err != nil {
						return err
					}

					// construct adapters
					accountsRepo := repos.NewAccountsRepo(db)
					usersRepo := repos.NewUsersRepo(db)
					passwordManager := pw.NewBCryptPasswordHasher()
					pkeyManager := gcrypto.NewRsaPKeyManager()
					txManager := dbAdapters.NewBunTxProvider(db)
					registerUser := users.NewRegisterUserUseCase(users.RegisterUserUseCaseConfig{
						TxProvider:           txManager,
						AccountsRepo:         accountsRepo,
						UsersRepo:            usersRepo,
						PasswordHashProvider: passwordManager,
						PKeyManager:          pkeyManager,
						LocalDomain:          config.Domain,
						Host:                 config.Host(),
					})

					_, derr := registerUser.RegisterUser(context.Background(), users.RegisterUserUseCaseInput{
						Email:    email,
						Username: validUsername,
						Password: password,
						Admin:    true,
					})
					if derr != nil {
						return derr
					}

					fmt.Printf("registered new user <%s> as admin", username)
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal("failed: ", err)
	}
}
