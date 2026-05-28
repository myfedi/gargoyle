package mastodon

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
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
type RemoteURLException struct {
	Host           string
	AllowHTTP      bool
	AllowPrivateIP bool
}

type RemoteAccountResolver struct {
	client     *http.Client
	exceptions []RemoteURLException
}

func NewRemoteAccountResolver(client *http.Client, exceptions []RemoteURLException) RemoteAccountResolver {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return RemoteAccountResolver{client: client, exceptions: exceptions}
}

func (r RemoteAccountResolver) ResolveAccount(ctx context.Context, query string, signer *models.Account) (*models.Account, error) {
	actorURL, err := r.actorURL(ctx, query)
	if err != nil {
		return nil, err
	}
	return r.fetchActor(ctx, actorURL, signer)
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
	if remoteURLExceptionForHost(parts[1], r.exceptions).AllowHTTP {
		scheme = "http"
	}
	wfURL := fmt.Sprintf("%s://%s/.well-known/webfinger?resource=%s", scheme, parts[1], url.QueryEscape("acct:"+query))
	if err := validateRemoteURL(ctx, wfURL, r.exceptions); err != nil {
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

func (r RemoteAccountResolver) fetchActor(ctx context.Context, actorURL string, signer *models.Account) (*models.Account, error) {
	if err := validateRemoteURL(ctx, actorURL, r.exceptions); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actorURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/activity+json")
	if signer != nil {
		signFederatedGet(req, *signer)
	}
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

func signFederatedGet(req *http.Request, account models.Account) {
	if account.PrivateKey == nil {
		return
	}
	block, _ := pem.Decode([]byte(*account.PrivateKey))
	if block == nil {
		return
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Date", date)
	signed := signatureString(req.Method, req.URL, map[string]string{"host": req.URL.Host, "date": date}, []string{"(request-target)", "host", "date"})
	hash := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return
	}
	req.Header.Set("Signature", fmt.Sprintf(`keyId="%s#main-key",algorithm="rsa-sha256",headers="(request-target) host date",signature="%s"`, account.URI, base64.StdEncoding.EncodeToString(sig)))
}

func signatureString(method string, u *url.URL, headers map[string]string, order []string) string {
	path := u.RequestURI()
	if path == "" {
		path = u.Path
	}
	lines := make([]string, 0, len(order))
	for _, h := range order {
		switch strings.ToLower(h) {
		case "(request-target)":
			lines = append(lines, fmt.Sprintf("(request-target): %s %s", strings.ToLower(method), path))
		default:
			lines = append(lines, fmt.Sprintf("%s: %s", strings.ToLower(h), headers[strings.ToLower(h)]))
		}
	}
	return strings.Join(lines, "\n")
}

func validateRemoteURL(ctx context.Context, raw string, exceptions []RemoteURLException) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	exception := remoteURLExceptionForHost(u.Hostname(), exceptions)
	if u.Scheme != "https" && !(exception.AllowHTTP && u.Scheme == "http") {
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
		if !exception.AllowPrivateIP && (ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsPrivate()) {
			return errors.New("remote URL resolves to private address")
		}
	}
	return nil
}

func remoteURLExceptionForHost(host string, exceptions []RemoteURLException) RemoteURLException {
	for _, exception := range exceptions {
		if strings.EqualFold(exception.Host, host) {
			return exception
		}
	}
	return RemoteURLException{}
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
