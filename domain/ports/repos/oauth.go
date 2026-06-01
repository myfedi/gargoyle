package repos

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type CreateOAuthApplicationInput struct {
	Name         string
	RedirectURI  string
	Scopes       string
	Website      string
	ClientID     string
	ClientSecret string
}

type CreateOAuthAccessTokenInput struct {
	ApplicationID string
	UserID        string
	TokenHash     string
	Scopes        string
	ExpiresAt     *time.Time
}

type CreateOAuthAuthorizationCodeInput struct {
	ApplicationID       string
	UserID              string
	CodeHash            string
	RedirectURI         string
	Scopes              string
	CodeChallenge       string
	CodeChallengeMethod string
	ExpiresAt           time.Time
}

// OAuthRepository stores Mastodon-compatible OAuth clients and bearer tokens.
type OAuthRepository interface {
	CreateApplication(ctx context.Context, tx *db.Tx, input CreateOAuthApplicationInput) (*models.OAuthApplication, error)
	GetApplicationByClientID(ctx context.Context, tx *db.Tx, clientID string) (*models.OAuthApplication, error)
	CreateAccessToken(ctx context.Context, tx *db.Tx, input CreateOAuthAccessTokenInput) (*models.OAuthAccessToken, error)
	GetAccessTokenByHash(ctx context.Context, tx *db.Tx, tokenHash string) (*models.OAuthAccessToken, error)
	CreateAuthorizationCode(ctx context.Context, tx *db.Tx, input CreateOAuthAuthorizationCodeInput) (*models.OAuthAuthorizationCode, error)
	GetAuthorizationCodeByHash(ctx context.Context, tx *db.Tx, codeHash string) (*models.OAuthAuthorizationCode, error)
	MarkAuthorizationCodeUsed(ctx context.Context, tx *db.Tx, id string, usedAt time.Time) error
}
