package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/uptrace/bun"
)

type BunTx struct {
	tx bun.Tx
}

func (b BunTx) Commit() error   { return b.tx.Commit() }
func (b BunTx) Rollback() error { return b.tx.Rollback() }
func (b BunTx) NewInsert() any  { return b.tx.NewInsert() }
func (b BunTx) NewSelect() any  { return b.tx.NewSelect() }
func (b BunTx) NewUpdate() any  { return b.tx.NewUpdate() }
func (b BunTx) NewDelete() any  { return b.tx.NewDelete() }
func (b BunTx) Unwrap() bun.Tx {
	return b.tx
}

type BunTxProvider struct {
	db bun.IDB
}

var _ db.TxProvider = BunTxProvider{}

func NewBunTxProvider(db bun.IDB) BunTxProvider {
	return BunTxProvider{db: db}
}

// FIXME: options provided as pointer to sql.TxOptions fails detection and crashes
func (b BunTxProvider) RunInTx(ctx context.Context, options interface{}, runIn func(ctx context.Context, tx db.Tx) error) error {
	var opts *sql.TxOptions

	if options != nil {
		txOptions, ok := options.(sql.TxOptions)
		if !ok {
			return fmt.Errorf("options not adhering to sql.TxOptions")
		}
		opts = &txOptions
	}
	return b.db.RunInTx(ctx, opts, func(ctx context.Context, tx bun.Tx) error {
		adapted := BunTx{tx: tx}
		return runIn(ctx, adapted)
	})
}
