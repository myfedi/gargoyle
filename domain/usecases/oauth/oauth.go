package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	derrors "github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type Config struct {
	OAuthRepo          repos.OAuthRepository
	UsersRepo          repos.UsersRepository
	AccountsRepo       repos.AccountsRepo
	FollowsRepo        repos.FollowsRepository
	NotesRepo          repos.NotesRepository
	PasswordHash       ports.PasswordHashProvider
	TxProvider         db.TxProvider
	AllowPasswordGrant bool
}

const accessTokenTTL = 30 * 24 * time.Hour
const defaultScopes = "read write follow"

var supportedScopes = map[string]bool{
	"read":                true,
	"read:accounts":       true,
	"read:blocks":         true,
	"read:bookmarks":      true,
	"read:favourites":     true,
	"read:filters":        true,
	"read:follows":        true,
	"read:lists":          true,
	"read:mutes":          true,
	"read:notifications":  true,
	"read:search":         true,
	"read:statuses":       true,
	"write":               true,
	"write:accounts":      true,
	"write:blocks":        true,
	"write:bookmarks":     true,
	"write:conversations": true,
	"write:favourites":    true,
	"write:filters":       true,
	"write:follows":       true,
	"write:lists":         true,
	"write:media":         true,
	"write:mutes":         true,
	"write:notifications": true,
	"write:reports":       true,
	"write:statuses":      true,
	"follow":              true,
}

type UseCase struct{ cfg Config }

type RegisterApplicationInput struct {
	Name        string
	RedirectURI string
	Scopes      string
	Website     string
}

type AuthorizeInput struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	Username            string
	Password            string
	CodeChallenge       string
	CodeChallengeMethod string
}

type AuthorizationDetailsInput struct {
	ClientID     string
	RedirectURI  string
	ResponseType string
	Scope        string
}

type AuthorizationDetails struct {
	ApplicationName string
	RedirectURI     string
	Scopes          []string
}

type IssueTokenInput struct {
	GrantType    string
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
	Scope        string
	Code         string
	RedirectURI  string
	CodeVerifier string
}

type IssuedToken struct {
	AccessToken string
	TokenType   string
	Scope       string
	CreatedAt   int64
	ExpiresIn   int64
}

type AccountStats struct {
	FollowersCount int
	FollowingCount int
	StatusesCount  int
}

type AuthenticatedUser struct {
	User    *models.User
	Account *models.Account
	Scopes  string
	Stats   AccountStats
}

func NewUseCase(cfg Config) UseCase {
	if cfg.OAuthRepo == nil {
		panic("oauth use case requires OAuthRepo")
	}
	if cfg.UsersRepo == nil {
		panic("oauth use case requires UsersRepo")
	}
	if cfg.AccountsRepo == nil {
		panic("oauth use case requires AccountsRepo")
	}
	if cfg.FollowsRepo == nil {
		panic("oauth use case requires FollowsRepo")
	}
	if cfg.NotesRepo == nil {
		panic("oauth use case requires NotesRepo")
	}
	if cfg.PasswordHash == nil {
		panic("oauth use case requires PasswordHashProvider")
	}
	if cfg.TxProvider == nil {
		panic("oauth use case requires TxProvider")
	}
	return UseCase{cfg: cfg}
}

func (u UseCase) RegisterApplication(ctx context.Context, input RegisterApplicationInput) (*models.OAuthApplication, *derrors.DomainError) {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.RedirectURI) == "" {
		return nil, derrors.New(derrors.ErrBadRequest, "client_name and redirect_uris are required")
	}
	scopes, derr := normalizeRequestedScopes(input.Scopes, defaultScopes)
	if derr != nil {
		return nil, derr
	}
	clientID, err := randomToken(32)
	if err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	clientSecret, err := randomToken(32)
	if err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	app, err := u.cfg.OAuthRepo.CreateApplication(ctx, nil, repos.CreateOAuthApplicationInput{Name: input.Name, RedirectURI: input.RedirectURI, Scopes: scopes, Website: input.Website, ClientID: clientID, ClientSecret: clientSecret})
	if err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	return app, nil
}

func (u UseCase) AuthorizationDetails(ctx context.Context, input AuthorizationDetailsInput) (*AuthorizationDetails, *derrors.DomainError) {
	if input.ResponseType != "" && input.ResponseType != "code" {
		return nil, derrors.New(derrors.ErrBadRequest, "unsupported response_type")
	}
	app, derr := u.validatedApplication(ctx, input.ClientID, input.RedirectURI)
	if derr != nil {
		return nil, derr
	}
	scope, derr := normalizeRequestedScopes(input.Scope, app.Scopes)
	if derr != nil {
		return nil, derr
	}
	if derr := ensureScopesAllowed(scope, app.Scopes); derr != nil {
		return nil, derr
	}
	return &AuthorizationDetails{ApplicationName: app.Name, RedirectURI: input.RedirectURI, Scopes: strings.Fields(scope)}, nil
}

func (u UseCase) Authorize(ctx context.Context, input AuthorizeInput) (string, *derrors.DomainError) {
	if input.ResponseType != "code" {
		return "", derrors.New(derrors.ErrBadRequest, "unsupported response_type")
	}
	app, derr := u.validatedApplication(ctx, input.ClientID, input.RedirectURI)
	if derr != nil {
		return "", derr
	}
	user, err := u.userByLogin(ctx, input.Username)
	if err != nil || u.cfg.PasswordHash.CompareHashAndPassword(user.PasswordHash, input.Password) != nil {
		return "", derrors.New(derrors.ErrUnauthorized, "invalid credentials")
	}
	scope, derr := normalizeRequestedScopes(input.Scope, app.Scopes)
	if derr != nil {
		return "", derr
	}
	if derr := ensureScopesAllowed(scope, app.Scopes); derr != nil {
		return "", derr
	}
	code, err := randomToken(32)
	if err != nil {
		return "", derrors.NewErr(derrors.ErrInternal, err)
	}
	_, err = u.cfg.OAuthRepo.CreateAuthorizationCode(ctx, nil, repos.CreateOAuthAuthorizationCodeInput{ApplicationID: app.ID, UserID: user.ID, CodeHash: TokenHash(code), RedirectURI: input.RedirectURI, Scopes: scope, CodeChallenge: input.CodeChallenge, CodeChallengeMethod: input.CodeChallengeMethod, ExpiresAt: time.Now().Add(10 * time.Minute)})
	if err != nil {
		return "", derrors.NewErr(derrors.ErrInternal, err)
	}
	redirect, err := url.Parse(input.RedirectURI)
	if err != nil {
		return "", derrors.NewErr(derrors.ErrBadRequest, err)
	}
	query := redirect.Query()
	query.Set("code", code)
	if input.State != "" {
		query.Set("state", input.State)
	}
	redirect.RawQuery = query.Encode()
	return redirect.String(), nil
}

func (u UseCase) IssueToken(ctx context.Context, input IssueTokenInput) (*IssuedToken, *derrors.DomainError) {
	app, err := u.cfg.OAuthRepo.GetApplicationByClientID(ctx, nil, input.ClientID)
	if err != nil {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid client_id")
	}
	if input.GrantType == "password" {
		if !u.cfg.AllowPasswordGrant {
			return nil, derrors.New(derrors.ErrBadRequest, "password grant is disabled")
		}
		if !constantTimeStringEqual(app.ClientSecret, input.ClientSecret) {
			return nil, derrors.New(derrors.ErrUnauthorized, "invalid client credentials")
		}
		return u.issuePasswordToken(ctx, app, input)
	}
	if input.GrantType == "authorization_code" {
		clientSecretProvided := input.ClientSecret != ""
		if clientSecretProvided && !constantTimeStringEqual(app.ClientSecret, input.ClientSecret) {
			return nil, derrors.New(derrors.ErrUnauthorized, "invalid client credentials")
		}
		return u.issueAuthorizationCodeToken(ctx, app, input, clientSecretProvided)
	}
	return nil, derrors.New(derrors.ErrBadRequest, "unsupported grant_type")
}

func (u UseCase) issuePasswordToken(ctx context.Context, app *models.OAuthApplication, input IssueTokenInput) (*IssuedToken, *derrors.DomainError) {
	user, err := u.userByLogin(ctx, input.Username)
	if err != nil {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid resource owner credentials")
	}
	if err := u.cfg.PasswordHash.CompareHashAndPassword(user.PasswordHash, input.Password); err != nil {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid resource owner credentials")
	}
	scope, derr := normalizeRequestedScopes(input.Scope, app.Scopes)
	if derr != nil {
		return nil, derr
	}
	if derr := ensureScopesAllowed(scope, app.Scopes); derr != nil {
		return nil, derr
	}
	return u.issueAccessToken(ctx, nil, app.ID, user.ID, scope)
}

func (u UseCase) issueAuthorizationCodeToken(ctx context.Context, app *models.OAuthApplication, input IssueTokenInput, clientSecretProvided bool) (*IssuedToken, *derrors.DomainError) {
	plain, err := randomToken(48)
	if err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(accessTokenTTL)
	var issued *IssuedToken
	err = u.cfg.TxProvider.RunInTx(ctx, nil, func(ctx context.Context, tx db.Tx) error {
		txPtr := &tx
		code, derr := u.validAuthorizationCode(ctx, txPtr, app, input, clientSecretProvided)
		if derr != nil {
			return derr
		}
		if err := u.cfg.OAuthRepo.MarkAuthorizationCodeUsed(ctx, txPtr, code.ID, time.Now().UTC()); err != nil {
			return derrors.New(derrors.ErrUnauthorized, "invalid authorization code")
		}
		if _, err := u.cfg.OAuthRepo.CreateAccessToken(ctx, txPtr, repos.CreateOAuthAccessTokenInput{ApplicationID: app.ID, UserID: code.UserID, TokenHash: TokenHash(plain), Scopes: code.Scopes, ExpiresAt: &expiresAt}); err != nil {
			return derrors.NewErr(derrors.ErrInternal, err)
		}
		issued = issuedTokenResponse(plain, code.Scopes, issuedAt)
		return nil
	})
	if err != nil {
		if derr, ok := err.(*derrors.DomainError); ok {
			return nil, derr
		}
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	return issued, nil
}

func (u UseCase) validAuthorizationCode(ctx context.Context, tx *db.Tx, app *models.OAuthApplication, input IssueTokenInput, clientSecretProvided bool) (*models.OAuthAuthorizationCode, *derrors.DomainError) {
	code, err := u.cfg.OAuthRepo.GetAuthorizationCodeByHash(ctx, tx, TokenHash(input.Code))
	if err != nil || invalidAuthorizationCode(code, app, input) {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid authorization code")
	}
	if !clientSecretProvided && code.CodeChallenge == "" {
		return nil, derrors.New(derrors.ErrUnauthorized, "public clients must use PKCE")
	}
	if !validPKCE(code.CodeChallenge, code.CodeChallengeMethod, input.CodeVerifier) {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid code verifier")
	}
	return code, nil
}

func invalidAuthorizationCode(code *models.OAuthAuthorizationCode, app *models.OAuthApplication, input IssueTokenInput) bool {
	return code.ApplicationID != app.ID || code.RedirectURI != input.RedirectURI || code.UsedAt != nil || time.Now().After(code.ExpiresAt)
}

func (u UseCase) issueAccessToken(ctx context.Context, tx *db.Tx, applicationID, userID, scope string) (*IssuedToken, *derrors.DomainError) {
	plain, err := randomToken(48)
	if err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(accessTokenTTL)
	if _, err := u.cfg.OAuthRepo.CreateAccessToken(ctx, tx, repos.CreateOAuthAccessTokenInput{ApplicationID: applicationID, UserID: userID, TokenHash: TokenHash(plain), Scopes: scope, ExpiresAt: &expiresAt}); err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	return issuedTokenResponse(plain, scope, issuedAt), nil
}

func issuedTokenResponse(plain, scope string, issuedAt time.Time) *IssuedToken {
	return &IssuedToken{AccessToken: plain, TokenType: "Bearer", Scope: scope, CreatedAt: issuedAt.Unix(), ExpiresIn: int64(accessTokenTTL.Seconds())}
}

func (u UseCase) AuthenticateBearer(ctx context.Context, bearer string) (*AuthenticatedUser, *derrors.DomainError) {
	if strings.TrimSpace(bearer) == "" {
		return nil, derrors.New(derrors.ErrUnauthorized, "missing bearer token")
	}
	token, err := u.cfg.OAuthRepo.GetAccessTokenByHash(ctx, nil, TokenHash(bearer))
	if err != nil {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid bearer token")
	}
	if token.ExpiresAt != nil && time.Now().UTC().After(*token.ExpiresAt) {
		return nil, derrors.New(derrors.ErrUnauthorized, "expired bearer token")
	}
	user, err := u.cfg.UsersRepo.GetUserByID(ctx, nil, token.UserID)
	if err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	account, err := u.cfg.AccountsRepo.GetAccountByUserID(ctx, nil, user.ID)
	if err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	stats, derr := u.accountStats(ctx, account)
	if derr != nil {
		return nil, derr
	}
	return &AuthenticatedUser{User: user, Account: account, Scopes: token.Scopes, Stats: stats}, nil
}

func (u UseCase) accountStats(ctx context.Context, account *models.Account) (AccountStats, *derrors.DomainError) {
	followers, err := u.cfg.FollowsRepo.CountFollowers(ctx, nil, account.ID)
	if err != nil {
		return AccountStats{}, derrors.NewErr(derrors.ErrInternal, err)
	}
	following, err := u.cfg.FollowsRepo.ListFollowing(ctx, nil, account.ID)
	if err != nil {
		return AccountStats{}, derrors.NewErr(derrors.ErrInternal, err)
	}
	statuses, err := u.cfg.NotesRepo.ListAttributedNotesPaged(ctx, nil, account.ID, account.URI, 0, "")
	if err != nil {
		return AccountStats{}, derrors.NewErr(derrors.ErrInternal, err)
	}
	return AccountStats{FollowersCount: followers, FollowingCount: len(following), StatusesCount: len(statuses)}, nil
}

func (u UseCase) userByLogin(ctx context.Context, login string) (*models.User, error) {
	if strings.Contains(login, "@") {
		if user, err := u.cfg.UsersRepo.GetUserByEmail(ctx, nil, login); err == nil {
			return user, nil
		}
	}
	return u.cfg.UsersRepo.GetUserByUsername(ctx, nil, login)
}

func (u UseCase) validatedApplication(ctx context.Context, clientID, redirectURI string) (*models.OAuthApplication, *derrors.DomainError) {
	app, err := u.cfg.OAuthRepo.GetApplicationByClientID(ctx, nil, clientID)
	if err != nil {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid client_id")
	}
	if !redirectURIMatches(app.RedirectURI, redirectURI) {
		return nil, derrors.New(derrors.ErrBadRequest, "redirect_uri is not registered")
	}
	return app, nil
}

func redirectURIMatches(registered, requested string) bool {
	for _, candidate := range strings.Fields(registered) {
		if candidate == requested {
			return true
		}
	}
	return false
}

func normalizeRequestedScopes(requested, fallback string) (string, *derrors.DomainError) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		requested = fallback
	}
	seen := map[string]bool{}
	scopes := []string{}
	for _, scope := range strings.Fields(requested) {
		if !supportedScopes[scope] {
			return "", derrors.New(derrors.ErrBadRequest, "unsupported OAuth scope")
		}
		if seen[scope] {
			continue
		}
		seen[scope] = true
		scopes = append(scopes, scope)
	}
	if len(scopes) == 0 {
		return "", derrors.New(derrors.ErrBadRequest, "at least one OAuth scope is required")
	}
	return strings.Join(scopes, " "), nil
}

func ensureScopesAllowed(requested, allowed string) *derrors.DomainError {
	allowedSet := map[string]bool{}
	for _, scope := range strings.Fields(allowed) {
		allowedSet[scope] = true
	}
	for _, scope := range strings.Fields(requested) {
		if allowedSet[scope] || parentScopeAllowed(scope, allowedSet) {
			continue
		}
		return derrors.New(derrors.ErrBadRequest, "requested OAuth scope is not registered for this application")
	}
	return nil
}

func parentScopeAllowed(scope string, allowedSet map[string]bool) bool {
	parent, _, ok := strings.Cut(scope, ":")
	return ok && allowedSet[parent]
}

func constantTimeStringEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func validPKCE(challenge, method, verifier string) bool {
	if challenge == "" {
		return true
	}
	if verifier == "" {
		return false
	}
	if method == "" || strings.EqualFold(method, "plain") {
		return verifier == challenge
	}
	if strings.EqualFold(method, "S256") {
		sum := sha256.Sum256([]byte(verifier))
		return base64.RawURLEncoding.EncodeToString(sum[:]) == challenge
	}
	return false
}

func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken(size int) (string, error) {
	if size <= 0 {
		return "", errors.New("token size must be positive")
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
