package repos

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type SocialRepository interface {
	CreateInteraction(ctx context.Context, tx *db.Tx, localAccountID, noteID, typ string) (*models.StatusInteraction, error)
	DeleteInteraction(ctx context.Context, tx *db.Tx, localAccountID, noteID, typ string) error
	InteractionExists(ctx context.Context, tx *db.Tx, localAccountID, noteID, typ string) (bool, error)
	CountInteractionsForNote(ctx context.Context, tx *db.Tx, noteID, typ string) (int, error)
	ListInteractions(ctx context.Context, tx *db.Tx, localAccountID, typ string, limit int) ([]models.StatusInteraction, error)
	CreateNotification(ctx context.Context, tx *db.Tx, localAccountID, actorAccountID, typ string, statusID *string) (*models.Notification, error)
	ListNotifications(ctx context.Context, tx *db.Tx, localAccountID string, limit int) ([]models.Notification, error)
	DeleteNotification(ctx context.Context, tx *db.Tx, localAccountID, notificationID string) error
	DeleteNotificationsByActorAndType(ctx context.Context, tx *db.Tx, localAccountID, actorAccountID, typ string) error
	ClearNotifications(ctx context.Context, tx *db.Tx, localAccountID string) error
}
