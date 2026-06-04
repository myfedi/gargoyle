package db

import (
	"database/sql"
	"net/url"
	"strings"

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
	sqlite, err := sql.Open(sqliteshim.ShimName, sqliteDSN(cfg.SqlitePath))
	if err != nil {
		panic(err)
	}

	// Create a Bun db on top of it.
	bun := bun.NewDB(sqlite, sqlitedialect.New())

	// Print all queries to stdout.
	if cfg.Debug {
		bun.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}

	if _, err := bun.Exec("PRAGMA foreign_keys = ON"); err != nil {
		panic(err)
	}
	if _, err := bun.Exec("PRAGMA journal_mode = WAL"); err != nil {
		panic(err)
	}
	if _, err := bun.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		panic(err)
	}
	sqlite.SetMaxOpenConns(4)
	sqlite.SetMaxIdleConns(4)

	return SqliteStore{
		Sqlite: sqlite,
		Bun:    bun,
	}
}

func sqliteDSN(path string) string {
	if path == "" {
		return path
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	params := url.Values{}
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "busy_timeout(5000)")
	return path + separator + params.Encode()
}
