package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type RemoteAccount struct {
	bun.BaseModel `bun:"table:remote_accounts"`

	ID                    string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt             time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	FetchedAt             time.Time `bun:"type:timestamptz,nullzero,notnull"`
	Username              string    `bun:",nullzero,notnull"`
	Domain                *string   `bun:",nullzero"`
	DisplayName           *string   `bun:",nullzero"`
	Summary               *string   `bun:",nullzero"`
	URI                   string    `bun:",nullzero,notnull,unique"`
	URL                   *string   `bun:",nullzero"`
	FieldsJSON            string    `bun:"profile_fields,nullzero,notnull,default:'[]'"`
	AvatarMediaID         *string   `bun:"type:CHAR(26),nullzero"`
	HeaderMediaID         *string   `bun:"type:CHAR(26),nullzero"`
	AvatarURL             *string   `bun:",nullzero"`
	HeaderURL             *string   `bun:",nullzero"`
	InboxURI              string    `bun:",nullzero,notnull"`
	OutboxURI             *string   `bun:",nullzero"`
	FollowingURI          string    `bun:",nullzero"`
	FollowersURI          string    `bun:",nullzero"`
	FeaturedCollectionURI string    `bun:",nullzero"`
	PublicKey             string    `bun:",nullzero"`
	ActorType             int       `bun:",nullzero,notnull"`
	Locked                bool      `bun:",notnull,default:false"`
}

func RemoteAccountFromModel(account models.Account) RemoteAccount {
	return RemoteAccount{ID: account.ID, FetchedAt: time.Now().UTC(), Username: account.Username, Domain: account.Domain, DisplayName: account.DisplayName, Summary: account.Summary, URI: account.URI, URL: account.URL, FieldsJSON: AccountProfileFieldsJSON(account.Fields), AvatarMediaID: account.AvatarMediaID, HeaderMediaID: account.HeaderMediaID, AvatarURL: account.AvatarURL, HeaderURL: account.HeaderURL, InboxURI: account.InboxURI, OutboxURI: account.OutboxURI, FollowingURI: account.FollowingURI, FollowersURI: account.FollowersURI, FeaturedCollectionURI: account.FeaturedCollectionURI, PublicKey: account.PublicKey, ActorType: int(account.ActorType), Locked: account.Locked}
}

func (a RemoteAccount) ToModel() models.Account {
	return models.Account{ID: a.ID, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, FetchedAt: a.FetchedAt, Username: a.Username, Domain: a.Domain, DisplayName: a.DisplayName, Summary: a.Summary, URI: a.URI, URL: a.URL, Fields: accountProfileFieldsFromJSON(a.FieldsJSON), AvatarMediaID: a.AvatarMediaID, HeaderMediaID: a.HeaderMediaID, AvatarURL: a.AvatarURL, HeaderURL: a.HeaderURL, InboxURI: a.InboxURI, OutboxURI: a.OutboxURI, FollowingURI: a.FollowingURI, FollowersURI: a.FollowersURI, FeaturedCollectionURI: a.FeaturedCollectionURI, PublicKey: a.PublicKey, ActorType: models.NewActorType(a.ActorType), Locked: a.Locked}
}
