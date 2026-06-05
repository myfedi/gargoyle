package clientapi

import (
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
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
	app.Post("/oauth/revoke", h.revokeToken)
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
	req := authorizeRequest{ClientID: c.Query("client_id"), RedirectURI: c.Query("redirect_uri"), ResponseType: c.Query("response_type", "code"), Scope: c.Query("scope"), State: c.Query("state"), CodeChallenge: c.Query("code_challenge"), CodeChallengeMethod: c.Query("code_challenge_method")}
	return h.renderAuthorizePage(c, req, "")
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
		if derr.Code == domainerrors.ErrUnauthorized {
			return h.renderAuthorizePage(c, req, "Check your credentials and try again.")
		}
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

type revokeTokenRequest struct {
	Token        string `json:"token" form:"token"`
	ClientID     string `json:"client_id" form:"client_id"`
	ClientSecret string `json:"client_secret" form:"client_secret"`
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
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderPragma, "no-cache")
	return c.JSON(tokenResponse{AccessToken: token.AccessToken, TokenType: token.TokenType, Scope: token.Scope, CreatedAt: token.CreatedAt, ExpiresIn: token.ExpiresIn})
}

func (h OAuthHandler) revokeToken(c *fiber.Ctx) error {
	var req revokeTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	if derr := h.uc.RevokeToken(c.UserContext(), oauth.RevokeTokenInput{ClientID: req.ClientID, ClientSecret: req.ClientSecret, Token: req.Token}); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderPragma, "no-cache")
	return c.JSON(fiber.Map{})
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
	return c.JSON(accountToResponseWithStats(principal.Account, principal.Stats))
}

func appToResponse(app *models.OAuthApplication) appResponse {
	return appResponse{ID: app.ID, Name: app.Name, Website: app.Website, RedirectURI: app.RedirectURI, ClientID: app.ClientID, ClientSecret: app.ClientSecret}
}

func accountToResponse(account *models.Account) accountResponse {
	return accountToResponseWithStats(account, oauth.AccountStats{})
}

func accountToResponseWithStats(account *models.Account, stats oauth.AccountStats) accountResponse {
	created := account.CreatedAt.UTC().Format(time.RFC3339)
	acct := account.Username
	if account.Domain != nil && *account.Domain != "" {
		acct = account.Username + "@" + *account.Domain
	}
	avatar := accountAvatarURL(account)
	header := accountHeaderURL(account)
	return accountResponse{ID: account.ID, Username: account.Username, Acct: acct, DisplayName: stringValue(account.DisplayName), Locked: account.Locked, Bot: false, Discoverable: true, Group: false, CreatedAt: created, Note: stringValue(account.Summary), URL: firstNonEmpty(stringValue(account.URL), account.URI), Avatar: avatar, AvatarStatic: avatar, Header: header, HeaderStatic: header, FollowersCount: stats.FollowersCount, FollowingCount: stats.FollowingCount, StatusesCount: stats.StatusesCount}
}

func accountAvatarURL(account *models.Account) string {
	if account.AvatarURL != nil && *account.AvatarURL != "" {
		return *account.AvatarURL
	}
	if account.AvatarMediaID == nil || *account.AvatarMediaID == "" {
		return ""
	}
	return accountMediaURL(account.URI, *account.AvatarMediaID)
}

func accountHeaderURL(account *models.Account) string {
	if account.HeaderURL != nil && *account.HeaderURL != "" {
		return *account.HeaderURL
	}
	if account.HeaderMediaID == nil || *account.HeaderMediaID == "" {
		return ""
	}
	return accountMediaURL(account.URI, *account.HeaderMediaID)
}

func accountMediaURL(actorURI, mediaID string) string {
	parsed, err := url.Parse(actorURI)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/media/" + strings.TrimLeft(mediaID, "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (h OAuthHandler) renderAuthorizePage(c *fiber.Ctx, req authorizeRequest, formError string) error {
	details, derr := h.uc.AuthorizationDetails(c.UserContext(), oauth.AuthorizationDetailsInput{ClientID: req.ClientID, RedirectURI: req.RedirectURI, ResponseType: req.ResponseType, Scope: req.Scope})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	scopeItems := ""
	for _, scope := range details.Scopes {
		scopeItems += `<li><span>` + htmlEscape(scopeLabel(scope)) + `</span><small>` + htmlEscape(scopeDescription(scope)) + `</small></li>`
	}
	errorHTML := ""
	if formError != "" {
		errorHTML = `<p class="error" role="alert">` + htmlEscape(formError) + `</p>`
	}
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderPragma, "no-cache")
	return c.SendString(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Authorize ` + htmlEscape(details.ApplicationName) + `</title>
<style>
:root { color-scheme: light; --bg: oklch(0.975 0.014 82); --panel: oklch(0.948 0.018 82); --text: oklch(0.235 0.018 55); --muted: oklch(0.505 0.018 55); --border: oklch(0.835 0.026 78); --accent: oklch(0.42 0.07 158); --accent-strong: oklch(0.35 0.075 158); --accent-text: oklch(0.975 0.014 82); --danger-bg: oklch(0.94 0.035 25); --danger: oklch(0.45 0.12 25); }
* { box-sizing: border-box; }
body { margin: 0; min-height: 100vh; display: grid; place-items: center; padding: 32px 18px; background: radial-gradient(circle at top left, oklch(0.93 0.034 92), transparent 34rem), var(--bg); color: var(--text); font: 15px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif; }
main { width: min(100%, 920px); display: grid; grid-template-columns: minmax(0, 1fr) 360px; gap: 22px; align-items: stretch; }
section, form { border: 1px solid var(--border); border-radius: 22px; background: color-mix(in oklch, var(--panel), var(--bg) 18%); box-shadow: 0 18px 60px color-mix(in oklch, var(--text), transparent 88%); }
section { padding: 30px; }
form { padding: 26px; background: oklch(0.965 0.012 82); }
.brand { margin: 0 0 22px; font-size: 13px; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; color: var(--accent-strong); }
h1 { margin: 0; max-width: 13ch; font-size: 34px; line-height: 1.08; letter-spacing: -0.035em; }
.lede { max-width: 58ch; margin: 18px 0 0; color: var(--muted); font-size: 16px; }
.redirect { margin: 22px 0 0; padding: 14px 16px; border: 1px solid var(--border); border-radius: 14px; background: var(--bg); color: var(--muted); overflow-wrap: anywhere; }
.redirect strong { display: block; margin-bottom: 3px; color: var(--text); font-size: 13px; }
ul { list-style: none; margin: 28px 0 0; padding: 0; display: grid; gap: 10px; }
li { display: grid; gap: 2px; padding: 13px 0; border-top: 1px solid var(--border); }
li span { font-weight: 700; }
li small { color: var(--muted); font-size: 13px; }
.form-title { margin: 0 0 4px; font-size: 20px; letter-spacing: -0.02em; }
.form-note { margin: 0 0 20px; color: var(--muted); }
label { display: grid; gap: 7px; margin-top: 14px; font-weight: 650; }
input { width: 100%; border: 1px solid var(--border); border-radius: 12px; padding: 12px 13px; background: var(--bg); color: var(--text); font: inherit; }
input:focus { outline: 2px solid color-mix(in oklch, var(--accent), transparent 40%); outline-offset: 2px; border-color: var(--accent); }
.actions { display: grid; gap: 10px; margin-top: 22px; }
button, .cancel { min-height: 44px; border-radius: 12px; font: inherit; font-weight: 750; text-align: center; text-decoration: none; }
button { border: 0; background: var(--accent); color: var(--accent-text); cursor: pointer; }
button:hover { background: var(--accent-strong); }
.cancel { display: grid; place-items: center; border: 1px solid var(--border); color: var(--text); }
.error { margin: 0 0 16px; padding: 11px 12px; border-radius: 12px; background: var(--danger-bg); color: var(--danger); font-weight: 650; }
.fine { margin: 18px 0 0; color: var(--muted); font-size: 12px; }
@media (max-width: 760px) { body { place-items: start center; } main { grid-template-columns: 1fr; } section, form { padding: 22px; border-radius: 18px; } h1 { max-width: none; font-size: 29px; } }
</style>
</head>
<body>
<main>
<section aria-labelledby="grant-title">
<p class="brand">Gargoyle authorization</p>
<h1 id="grant-title">Let ` + htmlEscape(details.ApplicationName) + ` use your account?</h1>
<p class="lede">Review what this app can do, then sign in with your local Gargoyle account to approve access.</p>
<p class="redirect"><strong>After approval, you will return to</strong>` + htmlEscape(details.RedirectURI) + `</p>
<ul aria-label="Requested access">` + scopeItems + `</ul>
</section>
<form method="post" action="/oauth/authorize">
<h2 class="form-title">Confirm access</h2>
<p class="form-note">Use your Gargoyle username or email. Your password is sent only to this server.</p>
` + errorHTML + `
<input type="hidden" name="client_id" value="` + htmlEscape(req.ClientID) + `">
<input type="hidden" name="redirect_uri" value="` + htmlEscape(req.RedirectURI) + `">
<input type="hidden" name="response_type" value="` + htmlEscape(req.ResponseType) + `">
<input type="hidden" name="scope" value="` + htmlEscape(req.Scope) + `">
<input type="hidden" name="state" value="` + htmlEscape(req.State) + `">
<input type="hidden" name="code_challenge" value="` + htmlEscape(req.CodeChallenge) + `">
<input type="hidden" name="code_challenge_method" value="` + htmlEscape(req.CodeChallengeMethod) + `">
<label>Username or email <input name="username" autocomplete="username" value="` + htmlEscape(req.Username) + `" required></label>
<label>Password <input name="password" type="password" autocomplete="current-password" required></label>
<div class="actions"><button type="submit">Approve access</button><a class="cancel" href="/">Cancel</a></div>
<p class="fine">Only approve applications you trust. You can clear your browser session from the Gargoyle UI.</p>
</form>
</main>
</body>
</html>`)
}

func scopeLabel(scope string) string {
	switch scope {
	case "read":
		return "Read your account"
	case "write":
		return "Post and manage content"
	case "follow":
		return "Manage follows"
	default:
		return strings.ReplaceAll(scope, ":", ": ")
	}
}

func scopeDescription(scope string) string {
	if strings.HasPrefix(scope, "read") {
		return "View account information, timelines, notifications, and related records."
	}
	if strings.HasPrefix(scope, "write") {
		return "Create or update posts, media, profile details, and client-side actions."
	}
	if scope == "follow" {
		return "Follow and unfollow accounts from this Gargoyle account."
	}
	return "Access requested by this application."
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
