package models

import "time"

// OAuthApplication is a Mastodon-compatible client application registration.
type OAuthApplication struct {
	ID           string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Name         string
	RedirectURI  string
	Scopes       string
	Website      string
	ClientID     string
	ClientSecret string
}

// OAuthAccessToken is an issued bearer token. TokenHash stores a SHA-256 hash of
// the bearer token; the plaintext token is only returned once at issue time.
type OAuthAccessToken struct {
	ID            string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ApplicationID string
	UserID        string
	TokenHash     string
	Scopes        string
	ExpiresAt     *time.Time
}

// OAuthAuthorizationCode is a short-lived browser OAuth grant exchanged for an
// access token by Mastodon-compatible clients.
type OAuthAuthorizationCode struct {
	ID                  string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ApplicationID       string
	UserID              string
	CodeHash            string
	RedirectURI         string
	Scopes              string
	CodeChallenge       string
	CodeChallengeMethod string
	ExpiresAt           time.Time
	UsedAt              *time.Time
}
