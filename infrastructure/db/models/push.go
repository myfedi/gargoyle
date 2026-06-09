package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type PushSubscription struct {
	bun.BaseModel      `bun:"table:push_subscriptions"`
	ID                 string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt          time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	LocalAccountID     string    `bun:"type:CHAR(26),nullzero,notnull"`
	AccessTokenID      string    `bun:"type:CHAR(26),nullzero,notnull,unique"`
	Endpoint           string    `bun:",nullzero,notnull"`
	KeyP256DH          string    `bun:",nullzero,notnull"`
	KeyAuth            string    `bun:",nullzero,notnull"`
	Policy             string    `bun:",nullzero,notnull,default:'all'"`
	AlertMention       bool      `bun:",notnull,default:true"`
	AlertStatus        bool      `bun:",notnull,default:false"`
	AlertReblog        bool      `bun:",notnull,default:true"`
	AlertFollow        bool      `bun:",notnull,default:true"`
	AlertFollowRequest bool      `bun:",notnull,default:true"`
	AlertFavourite     bool      `bun:",notnull,default:true"`
	AlertPoll          bool      `bun:",notnull,default:true"`
	AlertUpdate        bool      `bun:",notnull,default:false"`
	AlertAdminSignUp   bool      `bun:",notnull,default:false"`
	AlertAdminReport   bool      `bun:",notnull,default:false"`
}

func (s PushSubscription) ToModel() models.PushSubscription {
	return models.PushSubscription{ID: s.ID, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt, LocalAccountID: s.LocalAccountID, AccessTokenID: s.AccessTokenID, Endpoint: s.Endpoint, KeyP256DH: s.KeyP256DH, KeyAuth: s.KeyAuth, Policy: s.Policy, Alerts: models.PushAlerts{Mention: s.AlertMention, Status: s.AlertStatus, Reblog: s.AlertReblog, Follow: s.AlertFollow, FollowRequest: s.AlertFollowRequest, Favourite: s.AlertFavourite, Poll: s.AlertPoll, Update: s.AlertUpdate, AdminSignUp: s.AlertAdminSignUp, AdminReport: s.AlertAdminReport}}
}

type PushDeliveryJob struct {
	bun.BaseModel  `bun:"table:push_delivery_jobs"`
	ID             string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt      time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	SubscriptionID string    `bun:"type:CHAR(26),nullzero,notnull"`
	NotificationID string    `bun:"type:CHAR(26),nullzero,notnull"`
	Attempts       int       `bun:",notnull,default:0"`
	NextAttemptAt  time.Time `bun:"type:timestamptz,nullzero,notnull"`
	LastError      *string
	Status         string     `bun:",nullzero,notnull,default:'pending'"`
	DeliveredAt    *time.Time `bun:"type:timestamptz"`
}

func (j PushDeliveryJob) ToModel() models.PushDeliveryJob {
	return models.PushDeliveryJob{ID: j.ID, CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt, SubscriptionID: j.SubscriptionID, NotificationID: j.NotificationID, Attempts: j.Attempts, NextAttemptAt: j.NextAttemptAt, LastError: j.LastError, Status: models.JobStatus(j.Status), DeliveredAt: j.DeliveredAt}
}
