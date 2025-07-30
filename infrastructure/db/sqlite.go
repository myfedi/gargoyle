package db

import (
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bundebug"
)

type SqliteStoreConfig struct {
	Debug      bool
	SqlitePath string
}

type SqliteStore struct {
	Sqlite *sql.DB
	Bun    *bun.DB
}

func NewSqliteStore(cfg SqliteStoreConfig) SqliteStore {
	sqlite, err := sql.Open(sqliteshim.ShimName, cfg.SqlitePath)
	if err != nil {
		panic(err)
	}

	// Create a Bun db on top of it.
	bun := bun.NewDB(sqlite, sqlitedialect.New())

	// Print all queries to stdout.
	if cfg.Debug {
		bun.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}

	bun.Exec("PRAGMA foreign_keys = ON")

	return SqliteStore{
		Sqlite: sqlite,
		Bun:    bun,
	}
}
