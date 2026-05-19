package users

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
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
	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

type UsersWebHandlerConfig struct {
	AccountsRepo   repos.AccountsRepo
	ActivitiesRepo repos.ActivitiesRepository
	FollowsRepo    repos.FollowsRepository
	Serializer     activitypub.ActorSerializer
	HTTPClient     *http.Client
}

type UsersWebHandler struct {
	cfg     UsersWebHandlerConfig
	handler apUsecases.GetUserProfileUseCase
}

// NewWebfingerWebHandler creates a new Webfinger handler with the given dependencies.
func NewUsersWebHandler(cfg UsersWebHandlerConfig) *UsersWebHandler {
	handler := apUsecases.NewGetUserProfileUseCase(apUsecases.GetUserProfileUseCaseConfig{
		AccountsRepo: cfg.AccountsRepo,
		Serializer:   cfg.Serializer,
	})
	return &UsersWebHandler{
		cfg:     cfg,
		handler: handler,
	}
}

// SetupHostMeta initializes the hostmeta route for the Fiber application.
func (h *UsersWebHandler) SetupUserProfileHandler(app *fiber.App) {
	app.Get("/users/:username", func(c *fiber.Ctx) error {
		username := c.Params("username")
		if username == "" {
			return c.Status(fiber.StatusBadRequest).SendString("missing username")
		}

		profile, derr := h.handler.GetUserProfile(username)
		if derr != nil {
			return web.HandleDomainError(c, derr)
		}
		c.Set(fiber.HeaderContentType, "application/activity+json")
		return c.SendString(profile)
	})

	app.Get("/users/:username/outbox", h.outboxCollection)
	app.Post("/users/:username/outbox", h.createOutboxActivity)
	app.Get("/users/:username/followers", h.followersCollection)
	app.Get("/users/:username/following", h.emptyCollection(func(account models.Account) string {
		return account.FollowingURI
	}))

	app.Post("/users/:username/inbox", h.handleInboxActivity)
}

type orderedCollectionResponse struct {
	Context      string            `json:"@context"`
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	TotalItems   int               `json:"totalItems"`
	OrderedItems []json.RawMessage `json:"orderedItems,omitempty"`
}

type activityEnvelope struct {
	Context json.RawMessage `json:"@context,omitempty"`
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Actor   json.RawMessage `json:"actor"`
	Object  json.RawMessage `json:"object,omitempty"`
}

func (h *UsersWebHandler) emptyCollection(id func(models.Account) string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		account, derr := h.account(c.Params("username"))
		if derr != nil {
			return web.HandleDomainError(c, derr)
		}
		return sendActivityJSON(c, orderedCollectionResponse{
			Context:    "https://www.w3.org/ns/activitystreams",
			ID:         id(*account),
			Type:       "OrderedCollection",
			TotalItems: 0,
		})
	}
}

func (h *UsersWebHandler) outboxCollection(c *fiber.Ctx) error {
	account, derr := h.account(c.Params("username"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if h.cfg.ActivitiesRepo == nil {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrInternal, "activities repository not configured"))
	}

	activities, err := h.cfg.ActivitiesRepo.ListOutboxActivities(nil, account.ID)
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}

	items := make([]json.RawMessage, 0, len(activities))
	for _, activity := range activities {
		items = append(items, json.RawMessage(activity.RawJSON))
	}

	id := account.URI + "/outbox"
	if account.OutboxURI != nil {
		id = *account.OutboxURI
	}
	return sendActivityJSON(c, orderedCollectionResponse{
		Context:      "https://www.w3.org/ns/activitystreams",
		ID:           id,
		Type:         "OrderedCollection",
		TotalItems:   len(items),
		OrderedItems: items,
	})
}

func (h *UsersWebHandler) followersCollection(c *fiber.Ctx) error {
	account, derr := h.account(c.Params("username"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if h.cfg.FollowsRepo == nil {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrInternal, "follows repository not configured"))
	}

	followers, err := h.cfg.FollowsRepo.ListFollowers(nil, account.ID)
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}

	items := make([]json.RawMessage, 0, len(followers))
	for _, follower := range followers {
		items = append(items, json.RawMessage(fmt.Sprintf("%q", follower.RemoteActor)))
	}

	return sendActivityJSON(c, orderedCollectionResponse{
		Context:      "https://www.w3.org/ns/activitystreams",
		ID:           account.FollowersURI,
		Type:         "OrderedCollection",
		TotalItems:   len(items),
		OrderedItems: items,
	})
}

func (h *UsersWebHandler) handleInboxActivity(c *fiber.Ctx) error {
	account, derr := h.account(c.Params("username"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if h.cfg.ActivitiesRepo == nil {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrInternal, "activities repository not configured"))
	}

	raw := append([]byte(nil), c.Body()...)
	activity, derr := parseActivity(raw)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.verifyInboundSignature(c, raw, activity.Actor); derr != nil {
		return web.HandleDomainError(c, derr)
	}

	stored, err := h.cfg.ActivitiesRepo.CreateActivity(nil, repos.CreateActivityInput{
		LocalAccountID: account.ID,
		Direction:      models.ActivityDirectionInbox,
		Type:           activity.Type,
		Actor:          activity.Actor,
		Object:         activity.Object,
		RawJSON:        string(raw),
	})
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}

	if activity.Type == "Follow" && h.cfg.FollowsRepo != nil {
		if activity.Object != account.URI {
			return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrBadRequest, "follow object does not match local actor"))
		}
		inbox := activity.Inbox
		if inbox == "" {
			inbox = h.fetchActorInbox(activity.Actor)
		}
		var inboxPtr *string
		if inbox != "" {
			inboxPtr = &inbox
		}
		follow, err := h.cfg.FollowsRepo.CreateFollow(nil, repos.CreateFollowInput{
			LocalAccountID: account.ID,
			RemoteActor:    activity.Actor,
			RemoteInbox:    inboxPtr,
			ActivityID:     stored.ID,
		})
		if err != nil {
			return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
		}
		if err := h.cfg.FollowsRepo.AcceptFollow(nil, follow.ID); err != nil {
			return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
		}
		if inbox != "" {
			h.deliverAccept(*account, *follow, raw, inbox)
		}
	}

	return c.SendStatus(fiber.StatusAccepted)
}

func (h *UsersWebHandler) createOutboxActivity(c *fiber.Ctx) error {
	account, derr := h.account(c.Params("username"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if h.cfg.ActivitiesRepo == nil {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrInternal, "activities repository not configured"))
	}

	raw := append([]byte(nil), c.Body()...)
	activity, derr := parseActivity(raw)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if activity.Actor != "" && activity.Actor != account.URI {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrBadRequest, "activity actor does not match local actor"))
	}

	_, err := h.cfg.ActivitiesRepo.CreateActivity(nil, repos.CreateActivityInput{
		LocalAccountID: account.ID,
		Direction:      models.ActivityDirectionOutbox,
		Type:           activity.Type,
		Actor:          account.URI,
		Object:         activity.Object,
		RawJSON:        string(raw),
	})
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}

	if h.cfg.FollowsRepo != nil {
		followers, err := h.cfg.FollowsRepo.ListFollowers(nil, account.ID)
		if err != nil {
			return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
		}
		for _, follower := range followers {
			if follower.RemoteInbox != nil {
				h.deliverSigned(raw, *follower.RemoteInbox, *account)
			}
		}
	}

	return c.SendStatus(fiber.StatusCreated)
}

type parsedActivity struct {
	Type   string
	Actor  string
	Object string
	Inbox  string
}

func parseActivity(raw []byte) (parsedActivity, *domainerrors.DomainError) {
	var envelope activityEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return parsedActivity{}, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	if envelope.Type == "" {
		return parsedActivity{}, domainerrors.New(domainerrors.ErrBadRequest, "activity type is required")
	}

	actor, inbox, err := extractIDAndInbox(envelope.Actor)
	if err != nil {
		return parsedActivity{}, domainerrors.NewErr(domainerrors.ErrBadRequest, fmt.Errorf("invalid actor: %w", err))
	}
	object, _, err := extractIDAndInbox(envelope.Object)
	if len(envelope.Object) > 0 && err != nil {
		return parsedActivity{}, domainerrors.NewErr(domainerrors.ErrBadRequest, fmt.Errorf("invalid object: %w", err))
	}

	return parsedActivity{Type: envelope.Type, Actor: actor, Object: object, Inbox: inbox}, nil
}

func extractIDAndInbox(raw json.RawMessage) (string, string, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, "", nil
	}
	var obj struct {
		ID    string `json:"id"`
		Inbox string `json:"inbox"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", "", err
	}
	return obj.ID, obj.Inbox, nil
}

func sendActivityJSON(c *fiber.Ctx, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	c.Set(fiber.HeaderContentType, "application/activity+json")
	return c.Send(body)
}

type remoteActorDocument struct {
	Inbox     string `json:"inbox"`
	PublicKey struct {
		ID           string `json:"id"`
		Owner        string `json:"owner"`
		PublicKeyPem string `json:"publicKeyPem"`
	} `json:"publicKey"`
}

func (h *UsersWebHandler) fetchActorInbox(actor string) string {
	actorDoc, err := h.fetchActorDocument(actor)
	if err != nil {
		return ""
	}
	return actorDoc.Inbox
}

func (h *UsersWebHandler) fetchActorDocument(actor string) (remoteActorDocument, error) {
	if actor == "" {
		return remoteActorDocument{}, errors.New("empty actor")
	}
	client := h.cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet, actor, nil)
	if err != nil {
		return remoteActorDocument{}, err
	}
	req.Header.Set("Accept", "application/activity+json")
	resp, err := client.Do(req)
	if err != nil {
		return remoteActorDocument{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return remoteActorDocument{}, fmt.Errorf("actor fetch failed with status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return remoteActorDocument{}, err
	}
	var actorDoc remoteActorDocument
	if err := json.Unmarshal(body, &actorDoc); err != nil {
		return remoteActorDocument{}, err
	}
	return actorDoc, nil
}

func (h *UsersWebHandler) deliverAccept(account models.Account, follow models.Follow, followRaw []byte, inbox string) {
	accept := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       account.URI + "/accepts/" + follow.ID,
		"type":     "Accept",
		"actor":    account.URI,
		"object":   json.RawMessage(followRaw),
	}
	body, err := json.Marshal(accept)
	if err != nil {
		return
	}
	h.deliverSigned(body, inbox, account)
}

func (h *UsersWebHandler) deliverSigned(body []byte, inbox string, account models.Account) {
	client := h.cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodPost, inbox, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Accept", "application/activity+json")
	signOutboundRequest(req, body, account)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
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

func (h *UsersWebHandler) verifyInboundSignature(c *fiber.Ctx, body []byte, actor string) *domainerrors.DomainError {
	sigHeader := c.Get("Signature")
	if sigHeader == "" {
		return nil
	}

	params := parseSignatureHeader(sigHeader)
	sig, err := base64.StdEncoding.DecodeString(params["signature"])
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrUnauthorized, err)
	}
	headers := strings.Fields(params["headers"])
	if len(headers) == 0 {
		headers = []string{"date"}
	}

	digest := c.Get("Digest")
	if digest != "" && digest != digestHeader(body) {
		return domainerrors.New(domainerrors.ErrUnauthorized, "digest mismatch")
	}

	actorDoc, err := h.fetchActorDocument(actor)
	if err != nil || actorDoc.PublicKey.PublicKeyPem == "" {
		return domainerrors.New(domainerrors.ErrUnauthorized, "could not fetch actor public key")
	}
	pub, err := parseRSAPublicKey(actorDoc.PublicKey.PublicKeyPem)
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrUnauthorized, err)
	}

	u, _ := url.Parse(c.OriginalURL())
	signed := signatureString(c.Method(), u, map[string]string{
		"host":   c.Hostname(),
		"date":   c.Get("Date"),
		"digest": digest,
	}, headers)
	hash := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], sig); err != nil {
		return domainerrors.NewErr(domainerrors.ErrUnauthorized, err)
	}
	return nil
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

func (h *UsersWebHandler) account(username string) (*models.Account, *domainerrors.DomainError) {
	if username == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "missing username")
	}

	account, err := h.cfg.AccountsRepo.GetLocalAccountByUsername(nil, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainerrors.New(domainerrors.ErrNotFound, "no such username")
		}
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return account, nil
}
