package shared

import (
	"context"
	"net/http"
	"net/url"
)

type AppCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type Token struct {
	AccessToken string `json:"access_token"`
}

type Account struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Acct        string `json:"acct"`
	DisplayName string `json:"display_name"`
	Note        string `json:"note"`
	Avatar      string `json:"avatar"`
	Header      string `json:"header"`
}

type Status struct {
	ID               string            `json:"id"`
	Content          string            `json:"content"`
	Visibility       string            `json:"visibility"`
	Reblogged        bool              `json:"reblogged"`
	Favourited       bool              `json:"favourited"`
	Reblog           *Status           `json:"reblog"`
	Mentions         []Account         `json:"mentions"`
	MediaAttachments []MediaAttachment `json:"media_attachments"`
	Account          Account           `json:"account"`
}

type MediaAttachment struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

type Notification struct {
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Status *Status `json:"status"`
}

type SearchResponse struct {
	Accounts []Account `json:"accounts"`
	Statuses []Status  `json:"statuses"`
}

type StatusContext struct {
	Ancestors   []Status `json:"ancestors"`
	Descendants []Status `json:"descendants"`
}

func RegisterApp(ctx context.Context, client Client) (AppCredentials, *ResponseError) {
	var app AppCredentials
	resp, body, err := client.PostForm(ctx, "/api/v1/apps", "", url.Values{
		"client_name":   {"gargoyle integration tests"},
		"redirect_uris": {"urn:ietf:wg:oauth:2.0:oob"},
		"scopes":        {"read write follow"},
	}, &app)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return app, &ResponseError{Response: resp, Body: body, Err: err}
	}
	return app, nil
}

func PasswordToken(ctx context.Context, client Client, app AppCredentials, username, password string) (string, *ResponseError) {
	var token Token
	resp, body, err := client.PostForm(ctx, "/oauth/token", "", url.Values{
		"grant_type":    {"password"},
		"client_id":     {app.ClientID},
		"client_secret": {app.ClientSecret},
		"username":      {username},
		"password":      {password},
		"scope":         {"read write follow"},
		"redirect_uri":  {"urn:ietf:wg:oauth:2.0:oob"},
	}, &token)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &ResponseError{Response: resp, Body: body, Err: err}
	}
	return token.AccessToken, nil
}

func VerifyCredentials(ctx context.Context, client Client, bearer string) (Account, *ResponseError) {
	var account Account
	resp, body, err := client.GetJSON(ctx, "/api/v1/accounts/verify_credentials", bearer, &account)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return account, &ResponseError{Response: resp, Body: body, Err: err}
	}
	return account, nil
}

type ResponseError struct {
	Response *http.Response
	Body     string
	Err      error
}

func (e *ResponseError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Body
}
