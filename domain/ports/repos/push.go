package repos

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type UpsertPushSubscriptionInput struct {
	LocalAccountID string
	AccessTokenID  string
	Endpoint       string
	KeyP256DH      string
	KeyAuth        string
	Policy         string
	Alerts         models.PushAlerts
}

type PushSubscriptionRepository interface {
	UpsertPushSubscription(ctx context.Context, tx *db.Tx, input UpsertPushSubscriptionInput) (*models.PushSubscription, error)
	GetPushSubscriptionByToken(ctx context.Context, tx *db.Tx, accessTokenID string) (*models.PushSubscription, error)
	UpdatePushSubscription(ctx context.Context, tx *db.Tx, accessTokenID, policy string, alerts models.PushAlerts) (*models.PushSubscription, error)
	DeletePushSubscriptionByToken(ctx context.Context, tx *db.Tx, accessTokenID string) error
	DeletePushSubscription(ctx context.Context, tx *db.Tx, id string) error
}

type PushDeliveryJobsRepository interface {
	ClaimDuePushDeliveryJobs(ctx context.Context, tx *db.Tx, now time.Time, limit int) ([]models.PushDeliveryJob, error)
	MarkPushDeliveryJobDelivered(ctx context.Context, tx *db.Tx, id string, deliveredAt time.Time) error
	MarkPushDeliveryJobFailed(ctx context.Context, tx *db.Tx, id string, attempts int, nextAttemptAt time.Time, lastError string) error
	GetPushDeliveryPayload(ctx context.Context, tx *db.Tx, job models.PushDeliveryJob) (*models.PushSubscription, *models.Notification, error)
}
