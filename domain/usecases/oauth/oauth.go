package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

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

type UseCase struct{ cfg Config }

type RegisterApplicationInput struct {
	Name        string
	RedirectURI string
	Scopes      string
	Website     string
}

type IssueTokenInput struct {
	GrantType    string
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
	Scope        string
}

type IssuedToken struct {
	AccessToken string
	TokenType   string
	Scope       string
	CreatedAt   int64
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

func (u UseCase) IssueToken(ctx context.Context, input IssueTokenInput) (*IssuedToken, *derrors.DomainError) {
	if input.GrantType != "password" {
		return nil, derrors.New(derrors.ErrBadRequest, "unsupported grant_type")
	}
	app, err := u.cfg.OAuthRepo.GetApplicationByClientID(ctx, nil, input.ClientID)
	if err != nil || app.ClientSecret != input.ClientSecret {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid client credentials")
	}
	user, err := u.cfg.UsersRepo.GetUserByUsername(ctx, nil, input.Username)
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
	plain, err := randomToken(48)
	if err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	if _, err := u.cfg.OAuthRepo.CreateAccessToken(ctx, nil, repos.CreateOAuthAccessTokenInput{ApplicationID: app.ID, UserID: user.ID, TokenHash: TokenHash(plain), Scopes: scope}); err != nil {
		return nil, derrors.NewErr(derrors.ErrInternal, err)
	}
	return &IssuedToken{AccessToken: plain, TokenType: "Bearer", Scope: scope}, nil
}

func (u UseCase) AuthenticateBearer(ctx context.Context, bearer string) (*AuthenticatedUser, *derrors.DomainError) {
	if strings.TrimSpace(bearer) == "" {
		return nil, derrors.New(derrors.ErrUnauthorized, "missing bearer token")
	}
	token, err := u.cfg.OAuthRepo.GetAccessTokenByHash(ctx, nil, TokenHash(bearer))
	if err != nil {
		return nil, derrors.New(derrors.ErrUnauthorized, "invalid bearer token")
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
