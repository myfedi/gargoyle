package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
)

type Account struct {
	ID        string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	UserID    *string   `bun:"type:CHAR(26),nullzero,notnull,unique"`
	CreatedAt time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	FetchedAt time.Time `bun:"type:timestamptz,nullzero"`
	Username  string    `bun:",nullzero,notnull,unique:accounts_username_domain_uniq"`
	// null if local user
	Domain      *string `bun:",nullzero,unique:accounts_username_domain_uniq"`
	DisplayName *string `bun:",nullzero"`
	Summary     *string `bun:",nullzero"`
	URI         string  `bun:",nullzero,notnull,unique"`
	// null for local accounts
	URL      *string `bun:",nullzero"`
	InboxURI string  `bun:",nullzero"`
	// should be set, some implementations don't tho for service accounts
	OutboxURI             *string `bun:",nullzero"`
	FollowingURI          string  `bun:",nullzero"`
	FollowersURI          string  `bun:",nullzero"`
	FeaturedCollectionURI string  `bun:",nullzero"`
	PrivateKey            *string `bun:",nullzero"`
	PublicKey             string  `bun:",nullzero,notnull,unique"`
	ActorType             string  `bun:",nullzero,notnull"`
}

func (a *Account) ToModel() models.Account {
	actorType := models.ParseActorType(a.ActorType)

	return models.Account{
		ID:                    a.ID,
		UserID:                a.UserID,
		CreatedAt:             a.CreatedAt,
		UpdatedAt:             a.UpdatedAt,
		FetchedAt:             a.FetchedAt,
		Username:              a.Username,
		Domain:                a.Domain,
		DisplayName:           a.DisplayName,
		Summary:               a.Summary,
		URI:                   a.URI,
		URL:                   a.URL,
		InboxURI:              a.InboxURI,
		OutboxURI:             a.OutboxURI,
		FollowingURI:          a.FollowingURI,
		FollowersURI:          a.FollowersURI,
		FeaturedCollectionURI: a.FeaturedCollectionURI,
		PrivateKey:            a.PrivateKey,
		PublicKey:             a.PublicKey,
		ActorType:             actorType,
	}
}
