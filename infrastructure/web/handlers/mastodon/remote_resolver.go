package mastodon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
)

// RemoteAccountResolver discovers ActivityPub actors through WebFinger and
// actor fetches for Mastodon-compatible account search/follow endpoints.
type RemoteAccountResolver struct {
	client             *http.Client
	allowHTTPRemote    bool
	allowPrivateRemote bool
}

func NewRemoteAccountResolver(client *http.Client, allowHTTPRemote bool, allowPrivateRemote bool) RemoteAccountResolver {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return RemoteAccountResolver{client: client, allowHTTPRemote: allowHTTPRemote, allowPrivateRemote: allowPrivateRemote}
}

func (r RemoteAccountResolver) ResolveAccount(ctx context.Context, query string, signer *models.Account) (*models.Account, error) {
	actorURL, err := r.actorURL(ctx, query)
	if err != nil {
		return nil, err
	}
	return r.fetchActor(ctx, actorURL)
}

func (r RemoteAccountResolver) actorURL(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(strings.TrimPrefix(query, "@"))
	if strings.HasPrefix(query, "http://") || strings.HasPrefix(query, "https://") {
		return query, nil
	}
	parts := strings.Split(query, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", errors.New("account search requires @user@host or actor URL")
	}
	scheme := "https"
	if r.allowHTTPRemote {
		scheme = "http"
	}
	wfURL := fmt.Sprintf("%s://%s/.well-known/webfinger?resource=%s", scheme, parts[1], url.QueryEscape("acct:"+query))
	if err := validateRemoteURL(ctx, wfURL, r.allowHTTPRemote, r.allowPrivateRemote); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wfURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/jrd+json, application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("webfinger failed with status %d", resp.StatusCode)
	}
	var doc struct {
		Links []struct {
			Rel  string `json:"rel"`
			Type string `json:"type"`
			Href string `json:"href"`
		} `json:"links"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return "", err
	}
	for _, link := range doc.Links {
		if link.Rel == "self" && strings.Contains(link.Type, "activity") && link.Href != "" {
			return link.Href, nil
		}
	}
	return "", errors.New("webfinger response did not include ActivityPub actor")
}

func (r RemoteAccountResolver) fetchActor(ctx context.Context, actorURL string) (*models.Account, error) {
	if err := validateRemoteURL(ctx, actorURL, r.allowHTTPRemote, r.allowPrivateRemote); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actorURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/activity+json")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("actor fetch failed with status %d", resp.StatusCode)
	}
	var doc struct {
		ID                string `json:"id"`
		Type              string `json:"type"`
		PreferredUsername string `json:"preferredUsername"`
		Name              string `json:"name"`
		Summary           string `json:"summary"`
		URL               any    `json:"url"`
		Inbox             string `json:"inbox"`
		Outbox            string `json:"outbox"`
		Followers         string `json:"followers"`
		Following         string `json:"following"`
		PublicKey         struct {
			PublicKeyPem string `json:"publicKeyPem"`
		} `json:"publicKey"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, err
	}
	if doc.ID == "" || doc.Inbox == "" || doc.PreferredUsername == "" {
		return nil, errors.New("actor document is missing required fields")
	}
	domain := ""
	if parsed, err := url.Parse(doc.ID); err == nil {
		domain = parsed.Host
	}
	actorURLValue := stringURL(doc.URL)
	return &models.Account{ID: mastodonAccountID(doc.ID), Username: doc.PreferredUsername, Domain: stringPtr(domain), DisplayName: stringPtr(firstNonEmpty(doc.Name, doc.PreferredUsername)), Summary: stringPtr(doc.Summary), URI: doc.ID, URL: stringPtr(firstNonEmpty(actorURLValue, doc.ID)), InboxURI: doc.Inbox, OutboxURI: stringPtr(doc.Outbox), FollowingURI: doc.Following, FollowersURI: doc.Followers, PublicKey: doc.PublicKey.PublicKeyPem, ActorType: models.ActorTypePerson}, nil
}

func mastodonAccountID(actor string) string {
	return "remote:" + base64.RawURLEncoding.EncodeToString([]byte(actor))
}

func validateRemoteURL(ctx context.Context, raw string, allowHTTP bool, allowPrivate bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && !(allowHTTP && u.Scheme == "http") {
		return errors.New("unsupported remote URL scheme")
	}
	if u.Hostname() == "" || u.User != nil {
		return errors.New("invalid remote URL host")
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", u.Hostname())
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if !allowPrivate && (ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsPrivate()) {
			return errors.New("remote URL resolves to private address")
		}
	}
	return nil
}

func stringURL(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
