package repos

import (
	"context"

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
}

// OAuthRepository stores Mastodon-compatible OAuth clients and bearer tokens.
type OAuthRepository interface {
	CreateApplication(ctx context.Context, tx *db.Tx, input CreateOAuthApplicationInput) (*models.OAuthApplication, error)
	GetApplicationByClientID(ctx context.Context, tx *db.Tx, clientID string) (*models.OAuthApplication, error)
	CreateAccessToken(ctx context.Context, tx *db.Tx, input CreateOAuthAccessTokenInput) (*models.OAuthAccessToken, error)
	GetAccessTokenByHash(ctx context.Context, tx *db.Tx, tokenHash string) (*models.OAuthAccessToken, error)
}
