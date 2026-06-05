package activitypub

import (
	"bytes"
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

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/activitypub"
)

// httpActivityPubTransport is the infrastructure adapter for remote ActivityPub
// I/O: actor fetching, signed delivery, and inbound HTTP-signature checks. The
// web handler depends on the domain ports, not these concrete methods.
type RemoteURLException struct {
	Host           string
	AllowHTTP      bool
	AllowPrivateIP bool
}

type httpActivityPubTransport struct {
	client     *http.Client
	retries    int
	exceptions []RemoteURLException
}

func newHTTPActivityPubTransport(client *http.Client, retries int, exceptions []RemoteURLException) httpActivityPubTransport {
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
		client.Transport = publicOnlyTransport(exceptions)
	}
	if client.CheckRedirect == nil {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return validateRemoteURL(req.Context(), req.URL.String(), exceptions)
		}
	}
	return httpActivityPubTransport{client: client, retries: retries, exceptions: exceptions}
}

func publicOnlyTransport(exceptions []RemoteURLException) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		conn, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		remoteHost, _, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		ip := net.ParseIP(remoteHost)
		if ip == nil {
			_ = conn.Close()
			return nil, errors.New("remote connection address is not an IP")
		}
		if !remoteURLExceptionForHost(host, exceptions).AllowPrivateIP && !isPublicIP(ip) {
			_ = conn.Close()
			return nil, errors.New("remote URL dialed private address")
		}
		return conn, nil
	}
	return transport
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

func remoteURLExceptionForHost(host string, exceptions []RemoteURLException) RemoteURLException {
	for _, exception := range exceptions {
		if strings.EqualFold(exception.Host, host) {
			return exception
		}
	}
	return RemoteURLException{}
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
		return false
	}
	return true
}

type remoteActorDocument struct {
	Inbox     string `json:"inbox"`
	PublicKey struct {
		ID           string `json:"id"`
		Owner        string `json:"owner"`
		PublicKeyPem string `json:"publicKeyPem"`
	} `json:"publicKey"`
}

func (t httpActivityPubTransport) FetchActor(ctx context.Context, actor string, signer *models.Account) (*activitypub.RemoteActorDocument, error) {
	if actor == "" {
		return nil, errors.New("empty actor")
	}
	if err := validateRemoteURL(ctx, actor, t.exceptions); err != nil {
		return nil, err
	}
	client := t.client
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actor, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", contentTypeActivityJSON)
	if signer != nil {
		signOutboundRequest(req, nil, *signer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("actor fetch failed with status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var actorDoc remoteActorDocument
	if err := json.Unmarshal(body, &actorDoc); err != nil {
		return nil, err
	}
	return &activitypub.RemoteActorDocument{Inbox: actorDoc.Inbox, PublicKey: activitypub.RemoteActorPublicKey{ID: actorDoc.PublicKey.ID, Owner: actorDoc.PublicKey.Owner, PublicKeyPem: actorDoc.PublicKey.PublicKeyPem}}, nil
}

func (t httpActivityPubTransport) Deliver(ctx context.Context, body []byte, inbox string, account models.Account) error {
	if err := validateRemoteURL(ctx, inbox, t.exceptions); err != nil {
		return err
	}
	client := t.client
	retries := t.retries
	if retries < 1 {
		retries = 3
	}
	for attempt := 0; attempt < retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, inbox, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", contentTypeActivityJSON)
		req.Header.Set("Accept", contentTypeActivityJSON)
		signOutboundRequest(req, body, account)
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			err = fmt.Errorf("delivery failed with status %d", resp.StatusCode)
		}
		time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
	}
	return errors.New("delivery failed after retries")
}

func signOutboundRequest(req *http.Request, body []byte, account models.Account) {
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

	digest := digestHeader(body)
	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("Digest", digest)
	req.Header.Set("Date", date)

	signed := signatureString(req.Method, req.URL, map[string]string{
		"host":   req.URL.Host,
		"date":   date,
		"digest": digest,
	}, []string{"(request-target)", "host", "date", "digest"})
	hash := sha256.Sum256([]byte(signed))
	// ActivityPub HTTP Signatures commonly use "rsa-sha256", which maps to
	// RSASSA-PKCS1-v1_5 with SHA-256. RSA-PSS would be preferable for new
	// protocols, but would break compatibility with many federation peers.
	// This is a signature operation, not RSA encryption.
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:]) // NOSONAR
	if err != nil {
		return
	}

	req.Header.Set("Signature", fmt.Sprintf(`keyId="%s#main-key",algorithm="rsa-sha256",headers="(request-target) host date digest",signature="%s"`, account.URI, base64.StdEncoding.EncodeToString(sig)))
}

func (t httpActivityPubTransport) VerifyInbound(ctx context.Context, input activitypub.SignatureVerificationInput) *domainerrors.DomainError {
	sigHeader := input.Headers["signature"]
	if sigHeader == "" {
		if input.Required {
			return domainerrors.New(domainerrors.ErrUnauthorized, "missing signature")
		}
		return nil
	}

	params := parseSignatureHeader(sigHeader)
	if keyID := params["keyId"]; keyID == "" {
		return domainerrors.New(domainerrors.ErrUnauthorized, "missing keyId")
	}
	sig, err := base64.StdEncoding.DecodeString(params["signature"])
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrUnauthorized, err)
	}
	headers := strings.Fields(params["headers"])
	if len(headers) == 0 {
		headers = []string{"date"}
	}

	if derr := validateHTTPDate(input.Headers["date"]); derr != nil {
		return derr
	}

	digest := input.Headers["digest"]
	if input.Method == http.MethodPost || len(input.Body) > 0 {
		if digest == "" {
			return domainerrors.New(domainerrors.ErrUnauthorized, "missing digest")
		}
		if !signedHeaderIncludes(headers, "digest") {
			return domainerrors.New(domainerrors.ErrUnauthorized, "digest header is not signed")
		}
		if !strings.EqualFold(digest, digestHeader(input.Body)) {
			return domainerrors.New(domainerrors.ErrUnauthorized, "digest mismatch")
		}
	}

	actorDoc, err := t.FetchActor(ctx, input.Actor, input.LocalAccount)
	if err != nil || actorDoc.PublicKey.PublicKeyPem == "" {
		return domainerrors.New(domainerrors.ErrUnauthorized, "could not fetch actor public key")
	}
	if !validKeyID(params["keyId"], input.Actor, actorDoc) {
		return domainerrors.New(domainerrors.ErrUnauthorized, "signature keyId does not match actor")
	}
	pub, err := parseRSAPublicKey(actorDoc.PublicKey.PublicKeyPem)
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrUnauthorized, err)
	}

	headerValues := map[string]string{
		"host":                input.Host,
		"date":                input.Headers["date"],
		"digest":              digest,
		contentTypeHeaderName: input.Headers[contentTypeHeaderName],
	}
	signed := signatureString(input.Method, input.URL, headerValues, headers)
	hash := sha256.Sum256([]byte(signed))
	// ActivityPub HTTP Signatures commonly use "rsa-sha256", which maps to
	// RSASSA-PKCS1-v1_5 with SHA-256. RSA-PSS would be preferable for new
	// protocols, but would break compatibility with many federation peers.
	// This is a signature operation, not RSA encryption.
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], sig); err != nil { // NOSONAR
		return domainerrors.NewErr(domainerrors.ErrUnauthorized, err)
	}
	return nil
}

func signatureVerificationInput(c *fiber.Ctx, body []byte, actor string, account *models.Account, required bool) activitypub.SignatureVerificationInput {
	u, _ := url.Parse(c.OriginalURL())
	return activitypub.SignatureVerificationInput{
		Method:       c.Method(),
		URL:          u,
		Host:         c.Hostname(),
		Body:         body,
		Actor:        actor,
		LocalAccount: account,
		Required:     required,
		Headers: map[string]string{
			"signature":           c.Get("Signature"),
			"date":                c.Get("Date"),
			"digest":              c.Get("Digest"),
			contentTypeHeaderName: c.Get("Content-Type"),
		},
	}
}

func validateHTTPDate(value string) *domainerrors.DomainError {
	if value == "" {
		return domainerrors.New(domainerrors.ErrUnauthorized, "missing date")
	}
	parsed, err := http.ParseTime(value)
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrUnauthorized, err)
	}
	if time.Since(parsed) > 5*time.Minute || time.Until(parsed) > 5*time.Minute {
		return domainerrors.New(domainerrors.ErrUnauthorized, "date outside allowed clock skew")
	}
	return nil
}

func validKeyID(keyID, actor string, doc *activitypub.RemoteActorDocument) bool {
	if keyID == "" || actor == "" {
		return false
	}
	if doc.PublicKey.ID != "" && keyID == doc.PublicKey.ID {
		return true
	}
	if doc.PublicKey.Owner != "" && doc.PublicKey.Owner != actor {
		return false
	}
	return strings.HasPrefix(keyID, actor+"#")
}

func signedHeaderIncludes(headers []string, target string) bool {
	for _, header := range headers {
		if strings.EqualFold(header, target) {
			return true
		}
	}
	return false
}

func digestHeader(body []byte) string {
	sum := sha256.Sum256(body)
	return "SHA-256=" + base64.StdEncoding.EncodeToString(sum[:])
}

func parseSignatureHeader(header string) map[string]string {
	res := map[string]string{}
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		res[key] = strings.Trim(value, `"`)
	}
	return res
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

func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("invalid public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return rsaPub, nil
}
