package models

import "time"

type Account struct {
	ID                    string
	UserID                string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	FetchedAt             time.Time
	Username              string
	Domain                string
	DisplayName           string
	Summary               string
	URI                   string
	URL                   string
	InboxURI              string
	OutboxURI             string
	FollowingURI          string
	FollowersURI          string
	FeaturedCollectionURI string
	PrivateKey            *string
	PublicKey             string
	ActorType             ActorType
}
