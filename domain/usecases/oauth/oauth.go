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
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type Config struct {
	OAuthRepo    repos.OAuthRepository
	UsersRepo    repos.UsersRepository
	AccountsRepo repos.AccountsRepo
	PasswordHash ports.PasswordHashProvider
}

const accessTokenTTL = 30 * 24 * time.Hour

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

type AuthenticatedUser struct {
	User    *models.User
	Account *models.Account
	Scopes  string
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
	if cfg.PasswordHash == nil {
		panic("oauth use case requires PasswordHashProvider")
	}
	return UseCase{cfg: cfg}
}

func (u UseCase) RegisterApplication(ctx context.Context, input RegisterApplicationInput) (*models.OAuthApplication, *derrors.DomainError) {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.RedirectURI) == "" {
		return nil, derrors.New(derrors.ErrBadRequest, "client_name and redirect_uris are required")
	}
	scopes := strings.TrimSpace(input.Scopes)
	if scopes == "" {
		scopes = "read write follow"
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
	scope := strings.TrimSpace(input.Scope)
	if scope == "" {
		scope = app.Scopes
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
	scope := strings.TrimSpace(input.Scope)
	if scope == "" {
		scope = app.Scopes
	}
	return u.issueAccessToken(ctx, app.ID, user.ID, scope)
}

func (u UseCase) issueAuthorizationCodeToken(ctx context.Context, app *models.OAuthApplication, input IssueTokenInput, clientSecretProvided bool) (*IssuedToken, *derrors.DomainError) {
	code, err := u.cfg.OAuthRepo.GetAuthorizationCodeByHash(ctx, nil, TokenHash(input.Code))
	if err != nil || code.ApplicationID != app.ID || code.RedirectURI != input.RedirectURI || code.UsedAt != nil || time.Now().After(code.ExpiresAt) {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid authorization code")
	}
	if !clientSecretProvided && code.CodeChallenge == "" {
		return nil, derrors.New(derrors.ErrUnauthorized, "public clients must use PKCE")
	}
	if !validPKCE(code.CodeChallenge, code.CodeChallengeMethod, input.CodeVerifier) {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid code verifier")
	}
	if err := u.cfg.OAuthRepo.MarkAuthorizationCodeUsed(ctx, nil, code.ID, time.Now()); err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	return u.issueAccessToken(ctx, app.ID, code.UserID, code.Scopes)
}

func (u UseCase) issueAccessToken(ctx context.Context, applicationID string, userID string, scope string) (*IssuedToken, *derrors.DomainError) {
	plain, err := randomToken(48)
	if err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	expiresAt := time.Now().UTC().Add(accessTokenTTL)
	if _, err := u.cfg.OAuthRepo.CreateAccessToken(ctx, nil, repos.CreateOAuthAccessTokenInput{ApplicationID: applicationID, UserID: userID, TokenHash: TokenHash(plain), Scopes: scope, ExpiresAt: &expiresAt}); err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	return &IssuedToken{AccessToken: plain, TokenType: "Bearer", Scope: scope, CreatedAt: time.Now().Unix(), ExpiresIn: int64(accessTokenTTL.Seconds())}, nil
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
	return &AuthenticatedUser{User: user, Account: account, Scopes: token.Scopes}, nil
}

func (u UseCase) userByLogin(ctx context.Context, login string) (*models.User, error) {
	if strings.Contains(login, "@") {
		if user, err := u.cfg.UsersRepo.GetUserByEmail(ctx, nil, login); err == nil {
			return user, nil
		}
	}
	return u.cfg.UsersRepo.GetUserByUsername(ctx, nil, login)
}

func (u UseCase) validatedApplication(ctx context.Context, clientID string, redirectURI string) (*models.OAuthApplication, *derrors.DomainError) {
	app, err := u.cfg.OAuthRepo.GetApplicationByClientID(ctx, nil, clientID)
	if err != nil {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid client_id")
	}
	if !redirectURIMatches(app.RedirectURI, redirectURI) {
		return nil, derrors.New(derrors.ErrBadRequest, "redirect_uri is not registered")
	}
	return app, nil
}

func redirectURIMatches(registered string, requested string) bool {
	for _, candidate := range strings.Fields(registered) {
		if candidate == requested {
			return true
		}
	}
	return false
}

func constantTimeStringEqual(a string, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func validPKCE(challenge string, method string, verifier string) bool {
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
