package models

import "time"

type PushAlerts struct {
	Mention       bool
	Status        bool
	Reblog        bool
	Follow        bool
	FollowRequest bool
	Favourite     bool
	Poll          bool
	Update        bool
	AdminSignUp   bool
	AdminReport   bool
}

type PushSubscription struct {
	ID             string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LocalAccountID string
	AccessTokenID  string
	Endpoint       string
	KeyP256DH      string
	KeyAuth        string
	Policy         string
	Alerts         PushAlerts
}

type PushDeliveryJob struct {
	ID             string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	SubscriptionID string
	NotificationID string
	Attempts       int
	NextAttemptAt  time.Time
	LastError      *string
	Status         JobStatus
	DeliveredAt    *time.Time
}
