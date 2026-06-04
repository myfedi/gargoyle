package repos

import (
	"context"
	"errors"

	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	dbPorts "github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	dbModels "github.com/myfedi/gargoyle/infrastructure/db/models"
	"github.com/uptrace/bun"
)

type ConversationsRepo struct{ db bun.IDB }

func NewConversationsRepo(db *bun.DB) *ConversationsRepo { return &ConversationsRepo{db: db} }

var _ repos.ConversationsRepository = &ConversationsRepo{}

func (r *ConversationsRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New("internal error: unexpected tx implementation provided")
	}
	return adapted.Unwrap(), nil
}

func (r *ConversationsRepo) DismissConversation(ctx context.Context, tx *dbPorts.Tx, localAccountID, conversationID string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewInsert().Model(&dbModels.ConversationDismissal{LocalAccountID: localAccountID, ConversationID: conversationID}).On("CONFLICT DO NOTHING").Exec(ctx)
	return err
}

func (r *ConversationsRepo) ConversationDismissed(ctx context.Context, tx *dbPorts.Tx, localAccountID, conversationID string) (bool, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return false, err
	}
	return db.NewSelect().Model((*dbModels.ConversationDismissal)(nil)).Where("local_account_id = ?", localAccountID).Where("conversation_id = ?", conversationID).Exists(ctx)
}
