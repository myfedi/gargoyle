package mastodon

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/usecases/oauth"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

type OAuthHandler struct{ uc oauth.UseCase }

func NewOAuthHandler(uc oauth.UseCase) OAuthHandler { return OAuthHandler{uc: uc} }

func (h OAuthHandler) Setup(app *fiber.App) {
	app.Post("/api/v1/apps", h.registerApplication)
	app.Post("/oauth/token", h.issueToken)
	app.Get("/api/v1/accounts/verify_credentials", h.verifyCredentials)
}

type registerAppRequest struct {
	ClientName  string `json:"client_name" form:"client_name"`
	RedirectURI string `json:"redirect_uris" form:"redirect_uris"`
	Scopes      string `json:"scopes" form:"scopes"`
	Website     string `json:"website" form:"website"`
}

type appResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Website      string `json:"website,omitempty"`
	RedirectURI  string `json:"redirect_uri"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	VapidKey     string `json:"vapid_key,omitempty"`
}

func (h OAuthHandler) registerApplication(c *fiber.Ctx) error {
	var req registerAppRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	app, derr := h.uc.RegisterApplication(c.UserContext(), oauth.RegisterApplicationInput{Name: req.ClientName, RedirectURI: req.RedirectURI, Scopes: req.Scopes, Website: req.Website})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(appToResponse(app))
}

type tokenRequest struct {
	GrantType    string `json:"grant_type" form:"grant_type"`
	ClientID     string `json:"client_id" form:"client_id"`
	ClientSecret string `json:"client_secret" form:"client_secret"`
	Username     string `json:"username" form:"username"`
	Password     string `json:"password" form:"password"`
	Scope        string `json:"scope" form:"scope"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	CreatedAt   int64  `json:"created_at"`
}

func (h OAuthHandler) issueToken(c *fiber.Ctx) error {
	var req tokenRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	token, derr := h.uc.IssueToken(c.UserContext(), oauth.IssueTokenInput{GrantType: req.GrantType, ClientID: req.ClientID, ClientSecret: req.ClientSecret, Username: req.Username, Password: req.Password, Scope: req.Scope})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(tokenResponse{AccessToken: token.AccessToken, TokenType: token.TokenType, Scope: token.Scope, CreatedAt: time.Now().Unix()})
}

type accountResponse struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	Acct           string `json:"acct"`
	DisplayName    string `json:"display_name"`
	Locked         bool   `json:"locked"`
	Bot            bool   `json:"bot"`
	Discoverable   bool   `json:"discoverable"`
	Group          bool   `json:"group"`
	CreatedAt      string `json:"created_at"`
	Note           string `json:"note"`
	URL            string `json:"url"`
	Avatar         string `json:"avatar"`
	AvatarStatic   string `json:"avatar_static"`
	Header         string `json:"header"`
	HeaderStatic   string `json:"header_static"`
	FollowersCount int    `json:"followers_count"`
	FollowingCount int    `json:"following_count"`
	StatusesCount  int    `json:"statuses_count"`
}

func (h OAuthHandler) verifyCredentials(c *fiber.Ctx) error {
	auth := c.Get(fiber.HeaderAuthorization)
	bearer := strings.TrimPrefix(auth, "Bearer ")
	principal, derr := h.uc.AuthenticateBearer(c.UserContext(), bearer)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(accountToResponse(principal.Account))
}

func appToResponse(app *models.OAuthApplication) appResponse {
	return appResponse{ID: app.ID, Name: app.Name, Website: app.Website, RedirectURI: app.RedirectURI, ClientID: app.ClientID, ClientSecret: app.ClientSecret}
}

func accountToResponse(account *models.Account) accountResponse {
	created := account.CreatedAt.UTC().Format(time.RFC3339)
	return accountResponse{ID: account.ID, Username: account.Username, Acct: account.Username, DisplayName: stringValue(account.DisplayName), Locked: false, Bot: false, Discoverable: true, Group: false, CreatedAt: created, Note: stringValue(account.Summary), URL: stringValue(account.URL), Avatar: "", AvatarStatic: "", Header: "", HeaderStatic: ""}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
