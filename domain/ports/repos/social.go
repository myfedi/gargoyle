package repos

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type SocialRepository interface {
	CreateInteraction(ctx context.Context, tx *db.Tx, localAccountID string, noteID string, typ string) (*models.StatusInteraction, error)
	DeleteInteraction(ctx context.Context, tx *db.Tx, localAccountID string, noteID string, typ string) error
	InteractionExists(ctx context.Context, tx *db.Tx, localAccountID string, noteID string, typ string) (bool, error)
	ListInteractions(ctx context.Context, tx *db.Tx, localAccountID string, typ string, limit int) ([]models.StatusInteraction, error)
	CreateNotification(ctx context.Context, tx *db.Tx, localAccountID string, actorAccountID string, typ string, statusID *string) (*models.Notification, error)
	ListNotifications(ctx context.Context, tx *db.Tx, localAccountID string, limit int) ([]models.Notification, error)
	DeleteNotification(ctx context.Context, tx *db.Tx, localAccountID string, notificationID string) error
	ClearNotifications(ctx context.Context, tx *db.Tx, localAccountID string) error
}
