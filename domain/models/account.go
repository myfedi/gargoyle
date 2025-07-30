package models

import "time"

type Account struct {
	ID                    string
	UserID                *string // non-nil for local users only
	CreatedAt             time.Time
	UpdatedAt             time.Time
	FetchedAt             time.Time
	Username              string
	Domain                *string
	DisplayName           *string
	Summary               *string
	URI                   string
	URL                   *string
	InboxURI              string
	OutboxURI             *string
	FollowingURI          string
	FollowersURI          string
	FeaturedCollectionURI string
	PrivateKey            *string // non-nil for local users only
	PublicKey             string
	ActorType             ActorType
}
