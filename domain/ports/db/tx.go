package db

import "context"

type Tx interface {
	NewInsert() any
	NewSelect() any
	NewUpdate() any
	NewDelete() any
}

type TxProvider interface {
	RunInTx(ctx context.Context, options interface{}, runIn func(ctx context.Context, tx Tx) error) error
}
