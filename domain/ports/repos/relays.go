package repos

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type CreateRelaySubscriptionInput struct {
	ActorURI        string
	InboxURI        string
	CreatedByUserID string
}

type RelaySubscriptionsRepository interface {
	CreateRelaySubscription(ctx context.Context, tx *db.Tx, input CreateRelaySubscriptionInput) (*models.RelaySubscription, error)
	ListRelaySubscriptions(ctx context.Context, tx *db.Tx) ([]models.RelaySubscription, error)
	ListAcceptedRelaySubscriptions(ctx context.Context, tx *db.Tx) ([]models.RelaySubscription, error)
	GetRelaySubscriptionByActor(ctx context.Context, tx *db.Tx, actorURI string) (*models.RelaySubscription, error)
	GetRelaySubscriptionByID(ctx context.Context, tx *db.Tx, id string) (*models.RelaySubscription, error)
	MarkRelaySubscriptionAccepted(ctx context.Context, tx *db.Tx, actorURI string, acceptedAt time.Time) error
	DisableRelaySubscription(ctx context.Context, tx *db.Tx, actorURI string) error
	DeleteRelaySubscription(ctx context.Context, tx *db.Tx, actorURI string) error
}
