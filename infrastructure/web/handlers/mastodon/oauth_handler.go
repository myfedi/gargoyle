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
	app.Get("/oauth/authorize", h.authorizeForm)
	app.Post("/oauth/authorize", h.authorize)
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

func (h OAuthHandler) authorizeForm(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.SendString(`<!doctype html>
<html><head><title>Authorize Gargoyle</title></head><body>
<h1>Authorize application</h1>
<form method="post" action="/oauth/authorize">
<input type="hidden" name="client_id" value="` + htmlEscape(c.Query("client_id")) + `">
<input type="hidden" name="redirect_uri" value="` + htmlEscape(c.Query("redirect_uri")) + `">
<input type="hidden" name="response_type" value="` + htmlEscape(c.Query("response_type", "code")) + `">
<input type="hidden" name="scope" value="` + htmlEscape(c.Query("scope")) + `">
<input type="hidden" name="state" value="` + htmlEscape(c.Query("state")) + `">
<input type="hidden" name="code_challenge" value="` + htmlEscape(c.Query("code_challenge")) + `">
<input type="hidden" name="code_challenge_method" value="` + htmlEscape(c.Query("code_challenge_method")) + `">
<label>Username <input name="username" autocomplete="username"></label><br>
<label>Password <input name="password" type="password" autocomplete="current-password"></label><br>
<button type="submit">Authorize</button>
</form>
</body></html>`)
}

type authorizeRequest struct {
	ClientID            string `form:"client_id"`
	RedirectURI         string `form:"redirect_uri"`
	ResponseType        string `form:"response_type"`
	Scope               string `form:"scope"`
	State               string `form:"state"`
	Username            string `form:"username"`
	Password            string `form:"password"`
	CodeChallenge       string `form:"code_challenge"`
	CodeChallengeMethod string `form:"code_challenge_method"`
}

func (h OAuthHandler) authorize(c *fiber.Ctx) error {
	var req authorizeRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	redirect, derr := h.uc.Authorize(c.UserContext(), oauth.AuthorizeInput{ClientID: req.ClientID, RedirectURI: req.RedirectURI, ResponseType: req.ResponseType, Scope: req.Scope, State: req.State, Username: req.Username, Password: req.Password, CodeChallenge: req.CodeChallenge, CodeChallengeMethod: req.CodeChallengeMethod})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.Redirect(redirect, fiber.StatusFound)
}

type tokenRequest struct {
	GrantType    string `json:"grant_type" form:"grant_type"`
	ClientID     string `json:"client_id" form:"client_id"`
	ClientSecret string `json:"client_secret" form:"client_secret"`
	Username     string `json:"username" form:"username"`
	Password     string `json:"password" form:"password"`
	Scope        string `json:"scope" form:"scope"`
	Code         string `json:"code" form:"code"`
	RedirectURI  string `json:"redirect_uri" form:"redirect_uri"`
	CodeVerifier string `json:"code_verifier" form:"code_verifier"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (h OAuthHandler) issueToken(c *fiber.Ctx) error {
	var req tokenRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	token, derr := h.uc.IssueToken(c.UserContext(), oauth.IssueTokenInput{GrantType: req.GrantType, ClientID: req.ClientID, ClientSecret: req.ClientSecret, Username: req.Username, Password: req.Password, Scope: req.Scope, Code: req.Code, RedirectURI: req.RedirectURI, CodeVerifier: req.CodeVerifier})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(tokenResponse{AccessToken: token.AccessToken, TokenType: token.TokenType, Scope: token.Scope, CreatedAt: token.CreatedAt, ExpiresIn: token.ExpiresIn})
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
	acct := account.Username
	if account.Domain != nil && *account.Domain != "" {
		acct = account.Username + "@" + *account.Domain
	}
	return accountResponse{ID: account.ID, Username: account.Username, Acct: acct, DisplayName: stringValue(account.DisplayName), Locked: false, Bot: false, Discoverable: true, Group: false, CreatedAt: created, Note: stringValue(account.Summary), URL: stringValue(account.URL), Avatar: "", AvatarStatic: "", Header: "", HeaderStatic: ""}
}

func htmlEscape(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "\"", "&quot;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	return value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
