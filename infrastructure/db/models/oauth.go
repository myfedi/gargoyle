package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/uptrace/bun"
)

type OAuthApplication struct {
	bun.BaseModel `bun:"table:oauth_applications"`

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
	bun.BaseModel `bun:"table:oauth_access_tokens"`

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

type OAuthAuthorizationCode struct {
	bun.BaseModel `bun:"table:oauth_authorization_codes"`

	ID                  string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt           time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt           time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	ApplicationID       string    `bun:"type:CHAR(26),nullzero,notnull"`
	UserID              string    `bun:"type:CHAR(26),nullzero,notnull"`
	CodeHash            string    `bun:",nullzero,notnull,unique"`
	RedirectURI         string    `bun:",nullzero,notnull"`
	Scopes              string    `bun:",nullzero,notnull"`
	CodeChallenge       string
	CodeChallengeMethod string
	ExpiresAt           time.Time  `bun:"type:timestamptz,nullzero,notnull"`
	UsedAt              *time.Time `bun:"type:timestamptz"`
}

func (c OAuthAuthorizationCode) ToModel() models.OAuthAuthorizationCode {
	return models.OAuthAuthorizationCode{ID: c.ID, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, ApplicationID: c.ApplicationID, UserID: c.UserID, CodeHash: c.CodeHash, RedirectURI: c.RedirectURI, Scopes: c.Scopes, CodeChallenge: c.CodeChallenge, CodeChallengeMethod: c.CodeChallengeMethod, ExpiresAt: c.ExpiresAt, UsedAt: c.UsedAt}
}
