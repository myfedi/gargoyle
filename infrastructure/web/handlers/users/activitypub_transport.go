package users

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
type httpActivityPubTransport struct {
	client  *http.Client
	retries int
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
	client := t.client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actor, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/activity+json")
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

func (t httpActivityPubTransport) Deliver(ctx context.Context, body []byte, inbox string, account models.Account) {
	client := t.client
	if client == nil {
		client = http.DefaultClient
	}
	retries := t.retries
	if retries < 1 {
		retries = 3
	}
	for attempt := 0; attempt < retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, inbox, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/activity+json")
		req.Header.Set("Accept", "application/activity+json")
		signOutboundRequest(req, body, account)
		resp, err := client.Do(req)
		if err == nil && resp != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return
			}
		}
		time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
	}
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
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
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
	if digest != "" && !strings.EqualFold(digest, digestHeader(input.Body)) {
		return domainerrors.New(domainerrors.ErrUnauthorized, "digest mismatch")
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
		"host":         input.Host,
		"date":         input.Headers["date"],
		"digest":       digest,
		"content-type": input.Headers["content-type"],
	}
	signed := signatureString(input.Method, input.URL, headerValues, headers)
	hash := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], sig); err != nil {
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
			"signature":    c.Get("Signature"),
			"date":         c.Get("Date"),
			"digest":       c.Get("Digest"),
			"content-type": c.Get("Content-Type"),
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

func validKeyID(keyID string, actor string, doc *activitypub.RemoteActorDocument) bool {
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
