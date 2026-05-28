package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
)

type OAuthApplication struct {
	ID           string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt    time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt    time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	Name         string    `bun:",nullzero,notnull"`
	RedirectURI  string    `bun:",nullzero,notnull"`
	Scopes       string    `bun:",nullzero,notnull"`
	Website      string
	ClientID     string `bun:",nullzero,notnull,unique"`
	ClientSecret string `bun:",nullzero,notnull"`
}

func (a OAuthApplication) ToModel() models.OAuthApplication {
	return models.OAuthApplication{ID: a.ID, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, Name: a.Name, RedirectURI: a.RedirectURI, Scopes: a.Scopes, Website: a.Website, ClientID: a.ClientID, ClientSecret: a.ClientSecret}
}

type OAuthAccessToken struct {
	ID            string     `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt     time.Time  `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time  `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	ApplicationID string     `bun:"type:CHAR(26),nullzero,notnull"`
	UserID        string     `bun:"type:CHAR(26),nullzero,notnull"`
	TokenHash     string     `bun:",nullzero,notnull,unique"`
	Scopes        string     `bun:",nullzero,notnull"`
	ExpiresAt     *time.Time `bun:"type:timestamptz"`
}

func (t OAuthAccessToken) ToModel() models.OAuthAccessToken {
	return models.OAuthAccessToken{ID: t.ID, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, ApplicationID: t.ApplicationID, UserID: t.UserID, TokenHash: t.TokenHash, Scopes: t.Scopes, ExpiresAt: t.ExpiresAt}
}
