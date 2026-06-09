package clientapi

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
	"regexp"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
)

// RemoteAccountResolver discovers ActivityPub actors through WebFinger and
// actor fetches for Mastodon-compatible account search/follow endpoints.
const (
	remoteMaxProfileFields          = 4
	remoteMaxProfileFieldNameRunes  = 255
	remoteMaxProfileFieldValueRunes = 2047
)

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
	client = publicOnlyHTTPClient(client, exceptions)
	return RemoteAccountResolver{client: client, exceptions: exceptions}
}

// publicOnlyHTTPClient wraps the configured client with secure defaults. The
// dialer validates the IP address that was actually reached, not just DNS.
func publicOnlyHTTPClient(client *http.Client, exceptions []RemoteURLException) *http.Client {
	if client == nil {
		client = &http.Client{}
	} else {
		copy := *client
		client = &copy
	}
	if client.Timeout == 0 {
		client.Timeout = 10 * time.Second
	}

	if client.Transport == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DialContext = publicOnlyDialContext(exceptions)
		client.Transport = transport
	}
	return client
}

// publicOnlyDialContext enforces SSRF protections at connection time. Redirects
// and DNS changes cannot bypass this because the remote socket address is checked.
func publicOnlyDialContext(exceptions []RemoteURLException) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		conn, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}

		if err := validateDialedAddress(host, conn.RemoteAddr().String(), exceptions); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return conn, nil
	}
}

// validateDialedAddress rejects private IP ranges unless the host is explicitly
// configured as an exception. This is an infrastructure security boundary.
func validateDialedAddress(host, remoteAddress string, exceptions []RemoteURLException) error {
	remoteHost, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		return err
	}
	ip := net.ParseIP(remoteHost)
	if ip == nil {
		return errors.New("remote connection address is not an IP")
	}

	if !remoteURLExceptionForHost(host, exceptions).AllowPrivateIP && !isPublicIP(ip) {
		return errors.New("remote URL dialed private address")
	}
	return nil
}

func (r RemoteAccountResolver) ResolveAccount(ctx context.Context, query string, signer *models.Account) (*models.Account, error) {
	actorURL, err := r.actorURL(ctx, query)
	if err != nil {
		return nil, err
	}
	return r.fetchActor(ctx, actorURL, signer)
}

// actorURL accepts either a direct actor URL, a profile URL that can be reduced
// to a handle, or an @user@host-style handle resolved through WebFinger.
func (r RemoteAccountResolver) actorURL(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(strings.TrimPrefix(query, "@"))
	if strings.HasPrefix(query, "http://") || strings.HasPrefix(query, "https://") {
		if acct := accountHandleFromProfileURL(query); acct != "" {
			return r.actorURL(ctx, acct)
		}
		return query, nil
	}
	username, host, err := accountHandleParts(query)
	if err != nil {
		return "", err
	}

	return r.webfingerActorURL(ctx, username+"@"+host, host)
}

// accountHandleParts validates the client-facing account lookup syntax before
// any network request is attempted.
func accountHandleParts(query string) (string, string, error) {
	parts := strings.Split(query, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("account search requires @user@host or actor URL")
	}
	return parts[0], parts[1], nil
}

// webfingerActorURL resolves a handle to its ActivityPub actor URL while
// applying the same remote URL policy used for actor fetches.
func (r RemoteAccountResolver) webfingerActorURL(ctx context.Context, acct, host string) (string, error) {
	scheme := "https"
	if remoteURLExceptionForHost(host, r.exceptions).AllowHTTP {
		scheme = "http"
	}
	wfURL := fmt.Sprintf("%s://%s/.well-known/webfinger?resource=%s", scheme, host, url.QueryEscape("acct:"+acct))
	if err := validateRemoteURL(ctx, wfURL, r.exceptions); err != nil {
		return "", err
	}

	doc, err := r.fetchWebfinger(ctx, wfURL)
	if err != nil {
		return "", err
	}

	for _, link := range doc.Links {
		if link.Rel == "self" && strings.Contains(link.Type, "activity") && link.Href != "" {
			return link.Href, nil
		}
	}
	return "", errors.New("webfinger response did not include ActivityPub actor")
}

type webfingerDocument struct {
	Links []struct {
		Rel  string `json:"rel"`
		Type string `json:"type"`
		Href string `json:"href"`
	} `json:"links"`
}

// fetchWebfinger performs the HTTP request and bounded JSON decode. Parsing the
// selected ActivityPub link is kept separate for readability and testing.
func (r RemoteAccountResolver) fetchWebfinger(ctx context.Context, wfURL string) (*webfingerDocument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wfURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/jrd+json, application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("webfinger failed with status %d", resp.StatusCode)
	}

	var doc webfingerDocument
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// fetchActor downloads an ActivityPub actor and maps it into the domain Account
// model. HTTP details stay in this adapter; callers receive domain data only.
func (r RemoteAccountResolver) fetchActor(ctx context.Context, actorURL string, signer *models.Account) (*models.Account, error) {
	if err := validateRemoteURL(ctx, actorURL, r.exceptions); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actorURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", contentTypeActivityJSON)

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

	var doc remoteActorDocument
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, err
	}
	return accountFromRemoteActor(doc)
}

type remoteActorDocument struct {
	ID                string `json:"id"`
	Type              string `json:"type"`
	PreferredUsername string `json:"preferredUsername"`
	Name              string `json:"name"`
	Summary           string `json:"summary"`
	URL               any    `json:"url"`
	Icon              any    `json:"icon"`
	Image             any    `json:"image"`
	Attachment        any    `json:"attachment"`
	Inbox             string `json:"inbox"`
	Outbox            string `json:"outbox"`
	Followers         string `json:"followers"`
	Following         string `json:"following"`
	Locked            bool   `json:"manuallyApprovesFollowers"`
	PublicKey         struct {
		PublicKeyPem string `json:"publicKeyPem"`
	} `json:"publicKey"`
}

// accountFromRemoteActor is the adapter boundary from loosely typed remote JSON
// into the domain account model cached by repository ports.
func accountFromRemoteActor(doc remoteActorDocument) (*models.Account, error) {
	if doc.ID == "" || doc.Inbox == "" || doc.PreferredUsername == "" {
		return nil, errors.New("actor document is missing required fields")
	}
	domain := ""
	if parsed, err := url.Parse(doc.ID); err == nil {
		domain = parsed.Host
	}

	actorURLValue := stringURL(doc.URL)
	return &models.Account{
		ID:           mastodonAccountID(doc.ID),
		Username:     doc.PreferredUsername,
		Domain:       stringPtr(domain),
		DisplayName:  stringPtr(firstNonEmpty(doc.Name, doc.PreferredUsername)),
		Summary:      stringPtr(doc.Summary),
		URI:          doc.ID,
		URL:          stringPtr(firstNonEmpty(actorURLValue, doc.ID)),
		Fields:       accountProfileFields(doc.Attachment),
		AvatarURL:    stringPtr(stringURL(doc.Icon)),
		HeaderURL:    stringPtr(stringURL(doc.Image)),
		InboxURI:     doc.Inbox,
		OutboxURI:    stringPtr(doc.Outbox),
		FollowingURI: doc.Following,
		FollowersURI: doc.Followers,
		PublicKey:    doc.PublicKey.PublicKeyPem,
		ActorType:    models.ActorTypePerson,
		Locked:       doc.Locked,
	}, nil
}

func accountProfileFields(value any) []models.AccountProfileField {
	items, ok := value.([]any)
	if !ok {
		if value == nil {
			return nil
		}
		items = []any{value}
	}
	fields := make([]models.AccountProfileField, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok || anyString(object["type"]) != "PropertyValue" {
			continue
		}
		name := strings.TrimSpace(anyString(object["name"]))
		fieldValue := strings.TrimSpace(anyString(object["value"]))
		if name == "" && fieldValue == "" {
			continue
		}
		fields = append(fields, models.AccountProfileField{Name: trimRunes(name, remoteMaxProfileFieldNameRunes), Value: trimRunes(fieldValue, remoteMaxProfileFieldValueRunes)})
		if len(fields) == remoteMaxProfileFields {
			break
		}
	}
	return fields
}

func trimRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func mastodonAccountID(actor string) string {
	return remoteAccountIDPrefix + base64.RawURLEncoding.EncodeToString([]byte(actor))
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
	signed := signatureString(
		req.Method,
		req.URL,
		map[string]string{"host": req.URL.Host, "date": date},
		[]string{"(request-target)", "host", "date"},
	)
	hash := sha256.Sum256([]byte(signed))
	// ActivityPub HTTP Signatures commonly use "rsa-sha256", which maps to
	// RSASSA-PKCS1-v1_5 with SHA-256. RSA-PSS would be preferable for new
	// protocols, but would break compatibility with many federation peers.
	// This is a signature operation, not RSA encryption.
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:]) // NOSONAR
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
		if !exception.AllowPrivateIP && !isPublicIP(ip) {
			return errors.New("remote URL resolves to private address")
		}
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsMulticast() && !ip.IsLinkLocalMulticast() && !ip.IsLinkLocalUnicast() && !ip.IsPrivate()
}

func remoteURLExceptionForHost(host string, exceptions []RemoteURLException) RemoteURLException {
	for _, exception := range exceptions {
		if strings.EqualFold(exception.Host, host) {
			return exception
		}
	}
	return RemoteURLException{}
}

func accountHandleFromProfileURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	if match := regexp.MustCompile(`^/@([^/@\s]+)@([^/@\s]+)/?$`).FindStringSubmatch(parsed.Path); len(match) == 3 {
		return match[1] + "@" + match[2]
	}
	if match := regexp.MustCompile(`^/(?:@|users/)([^/@\s]+)/?$`).FindStringSubmatch(parsed.Path); len(match) == 2 {
		return match[1] + "@" + parsed.Hostname()
	}
	return ""
}

func anyString(value any) string {
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

func stringURL(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]any:
		return stringURL(v["url"])
	case []any:
		if len(v) > 0 {
			return stringURL(v[0])
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
