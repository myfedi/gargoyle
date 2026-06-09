package oauth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

func TestNormalizeRequestedScopesPreservesOrderAndRejectsUnknown(t *testing.T) {
	scopes, derr := normalizeRequestedScopes("write read read follow", defaultScopes)
	if derr != nil {
		t.Fatalf("normalize scopes: %v", derr)
	}
	if scopes != "write read follow" {
		t.Fatalf("expected scopes to preserve request order and dedupe, got %q", scopes)
	}

	if _, derr := normalizeRequestedScopes("read admin", defaultScopes); derr == nil {
		t.Fatalf("expected unsupported scope to fail")
	}
}

func TestEnsureScopesAllowedAllowsRegisteredParentScope(t *testing.T) {
	if derr := ensureScopesAllowed("read:statuses write:media", "read write"); derr != nil {
		t.Fatalf("parent scopes should allow narrower scopes: %v", derr)
	}

	derr := ensureScopesAllowed("write", "read follow")
	if derr == nil || !strings.Contains(derr.Message, "not registered") {
		t.Fatalf("expected unregistered write scope to fail, got %v", derr)
	}
}

func TestIssueTokenSupportsClientCredentialsGrant(t *testing.T) {
	repo := newTestOAuthRepo()
	repo.app.Scopes = "read write push"
	uc := UseCase{cfg: Config{OAuthRepo: repo}}

	issued, derr := uc.IssueToken(context.Background(), IssueTokenInput{GrantType: "client_credentials", ClientID: "client", ClientSecret: "secret", Scope: "read push"})
	if derr != nil {
		t.Fatalf("issue client credentials token: %v", derr)
	}
	if issued.AccessToken == "" || issued.Scope != "read push" {
		t.Fatalf("unexpected issued token: %+v", issued)
	}
	stored := repo.tokens[TokenHash(issued.AccessToken)]
	if stored == nil {
		t.Fatalf("expected token to be stored")
	}
	if stored.UserID != "" {
		t.Fatalf("client credentials token should not be user-bound, got %q", stored.UserID)
	}
}

func TestRevokeTokenDeletesTokenByHashAndIsIdempotent(t *testing.T) {
	repo := newTestOAuthRepo()
	uc := UseCase{cfg: Config{OAuthRepo: repo}}

	derr := uc.RevokeToken(context.Background(), RevokeTokenInput{ClientID: "client", ClientSecret: "secret", Token: "plain-token"})
	if derr != nil {
		t.Fatalf("revoke token: %v", derr)
	}
	if _, ok := repo.tokens[TokenHash("plain-token")]; ok {
		t.Fatalf("expected token hash to be deleted")
	}

	derr = uc.RevokeToken(context.Background(), RevokeTokenInput{ClientID: "client", ClientSecret: "secret", Token: "plain-token"})
	if derr != nil {
		t.Fatalf("second revoke should be idempotent: %v", derr)
	}
}

func TestRevokeTokenAllowsPublicPKCEClientWithoutClientSecret(t *testing.T) {
	repo := newTestOAuthRepo()
	uc := UseCase{cfg: Config{OAuthRepo: repo}}

	derr := uc.RevokeToken(context.Background(), RevokeTokenInput{ClientID: "client", Token: "plain-token"})
	if derr != nil {
		t.Fatalf("public client revoke should succeed: %v", derr)
	}
	if _, ok := repo.tokens[TokenHash("plain-token")]; ok {
		t.Fatalf("expected token hash to be deleted")
	}
}

func TestRevokeTokenRejectsInvalidClientSecretAndKeepsToken(t *testing.T) {
	repo := newTestOAuthRepo()
	uc := UseCase{cfg: Config{OAuthRepo: repo}}

	derr := uc.RevokeToken(context.Background(), RevokeTokenInput{ClientID: "client", ClientSecret: "wrong", Token: "plain-token"})
	if derr == nil || derr.Code != domainerrors.ErrUnauthorized {
		t.Fatalf("expected unauthorized for invalid client secret, got %v", derr)
	}
	if _, ok := repo.tokens[TokenHash("plain-token")]; !ok {
		t.Fatalf("token should remain after invalid client credentials")
	}
}

func TestRevokeTokenRequiresToken(t *testing.T) {
	repo := newTestOAuthRepo()
	uc := UseCase{cfg: Config{OAuthRepo: repo}}

	derr := uc.RevokeToken(context.Background(), RevokeTokenInput{ClientID: "client", ClientSecret: "secret"})
	if derr == nil || derr.Code != domainerrors.ErrBadRequest {
		t.Fatalf("expected bad request for missing token, got %v", derr)
	}
}

type testOAuthRepo struct {
	app    *models.OAuthApplication
	tokens map[string]*models.OAuthAccessToken
}

func newTestOAuthRepo() *testOAuthRepo {
	return &testOAuthRepo{
		app:    &models.OAuthApplication{ID: "app", ClientID: "client", ClientSecret: "secret"},
		tokens: map[string]*models.OAuthAccessToken{TokenHash("plain-token"): {ApplicationID: "app"}},
	}
}

func (r *testOAuthRepo) CreateApplication(context.Context, *db.Tx, repos.CreateOAuthApplicationInput) (*models.OAuthApplication, error) {
	return nil, errors.New("not implemented")
}

func (r *testOAuthRepo) GetApplicationByClientID(_ context.Context, _ *db.Tx, clientID string) (*models.OAuthApplication, error) {
	if clientID != r.app.ClientID {
		return nil, errors.New("not found")
	}
	return r.app, nil
}

func (r *testOAuthRepo) CreateAccessToken(_ context.Context, _ *db.Tx, input repos.CreateOAuthAccessTokenInput) (*models.OAuthAccessToken, error) {
	token := &models.OAuthAccessToken{ID: "token", ApplicationID: input.ApplicationID, UserID: input.UserID, TokenHash: input.TokenHash, Scopes: input.Scopes, ExpiresAt: input.ExpiresAt}
	r.tokens[input.TokenHash] = token
	return token, nil
}

func (r *testOAuthRepo) GetAccessTokenByHash(_ context.Context, _ *db.Tx, tokenHash string) (*models.OAuthAccessToken, error) {
	token, ok := r.tokens[tokenHash]
	if !ok {
		return nil, errors.New("not found")
	}
	return token, nil
}

func (r *testOAuthRepo) DeleteAccessTokenByHash(_ context.Context, _ *db.Tx, tokenHash string) error {
	delete(r.tokens, tokenHash)
	return nil
}

func (r *testOAuthRepo) CreateAuthorizationCode(context.Context, *db.Tx, repos.CreateOAuthAuthorizationCodeInput) (*models.OAuthAuthorizationCode, error) {
	return nil, errors.New("not implemented")
}

func (r *testOAuthRepo) GetAuthorizationCodeByHash(context.Context, *db.Tx, string) (*models.OAuthAuthorizationCode, error) {
	return nil, errors.New("not implemented")
}

func (r *testOAuthRepo) MarkAuthorizationCodeUsed(context.Context, *db.Tx, string, time.Time) error {
	return errors.New("not implemented")
}
