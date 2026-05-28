package repos

import (
	"context"

	"github.com/myfedi/gargoyle/domain/ports/db"
)

type ConversationsRepository interface {
	DismissConversation(ctx context.Context, tx *db.Tx, localAccountID string, conversationID string) error
	ConversationDismissed(ctx context.Context, tx *db.Tx, localAccountID string, conversationID string) (bool, error)
}
