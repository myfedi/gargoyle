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
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
	dbUtils "github.com/myfedi/gargoyle/infrastructure/db"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

type UsersWebHandlerConfig struct {
	TxProvider         db.TxProvider
	AccountsRepo       repos.AccountsRepo
	ActivitiesRepo     repos.ActivitiesRepository
	FollowsRepo        repos.FollowsRepository
	NotesRepo          repos.NotesRepository
	Serializer         activitypub.ActorSerializer
	HTTPClient         *http.Client
	RequireSignedInbox bool
	DeliveryRetries    int
}

type UsersWebHandler struct {
	cfg                    UsersWebHandlerConfig
	handler                apUsecases.GetUserProfileUseCase
	getOutbox              apUsecases.GetOutboxUseCase
	getFollowers           apUsecases.GetFollowersUseCase
	getFollowing           apUsecases.GetFollowingUseCase
	createFollowingUC      apUsecases.CreateFollowingUseCase
	createOutboxActivityUC apUsecases.CreateOutboxActivityUseCase
	handleInboxActivityUC  apUsecases.HandleInboxActivityUseCase
}

// NewWebfingerWebHandler creates a new Webfinger handler with the given dependencies.
func NewUsersWebHandler(cfg UsersWebHandlerConfig) *UsersWebHandler {
	handler := apUsecases.NewGetUserProfileUseCase(apUsecases.GetUserProfileUseCaseConfig{
		AccountsRepo: cfg.AccountsRepo,
		Serializer:   cfg.Serializer,
	})
	flowCfg := apUsecases.ActivityPubFlowConfig{
		TxProvider:     cfg.TxProvider,
		AccountsRepo:   cfg.AccountsRepo,
		ActivitiesRepo: cfg.ActivitiesRepo,
		FollowsRepo:    cfg.FollowsRepo,
		NotesRepo:      cfg.NotesRepo,
	}
	return &UsersWebHandler{
		cfg:                    cfg,
		handler:                handler,
		getOutbox:              apUsecases.NewGetOutboxUseCase(flowCfg),
		getFollowers:           apUsecases.NewGetFollowersUseCase(flowCfg),
		getFollowing:           apUsecases.NewGetFollowingUseCase(flowCfg),
		createFollowingUC:      apUsecases.NewCreateFollowingUseCase(flowCfg),
		createOutboxActivityUC: apUsecases.NewCreateOutboxActivityUseCase(flowCfg),
		handleInboxActivityUC:  apUsecases.NewHandleInboxActivityUseCase(flowCfg),
	}
}

// SetupHostMeta initializes the hostmeta route for the Fiber application.
func (h *UsersWebHandler) SetupUserProfileHandler(app *fiber.App) {
	app.Get("/users/:username", func(c *fiber.Ctx) error {
		username := c.Params("username")
		if username == "" {
			return c.Status(fiber.StatusBadRequest).SendString("missing username")
		}

		profile, derr := h.handler.GetUserProfile(c.UserContext(), username)
		if derr != nil {
			return web.HandleDomainError(c, derr)
		}
		c.Set(fiber.HeaderContentType, "application/activity+json")
		return c.SendString(profile)
	})

	app.Get("/users/:username/outbox", h.outboxCollection)
	app.Post("/users/:username/outbox", h.createOutboxActivity)
	app.Get("/users/:username/followers", h.followersCollection)
	app.Get("/users/:username/following", h.followingCollection)
	app.Post("/users/:username/following", h.createFollowing)

	app.Post("/users/:username/inbox", h.handleInboxActivity)
}

type orderedCollectionResponse struct {
	Context      string            `json:"@context"`
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	TotalItems   int               `json:"totalItems"`
	First        string            `json:"first,omitempty"`
	PartOf       string            `json:"partOf,omitempty"`
	OrderedItems []json.RawMessage `json:"orderedItems,omitempty"`
}

func (h *UsersWebHandler) outboxCollection(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	res, derr := h.getOutbox.GetOutbox(c.UserContext(), c.Params("username"), apUsecases.PaginationInput{Limit: limit, Offset: offset})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}

	items := make([]json.RawMessage, 0, len(res.Activities))
	for _, activity := range res.Activities {
		items = append(items, json.RawMessage(activity.RawJSON))
	}

	id := res.Account.URI + "/outbox"
	if res.Account.OutboxURI != nil {
		id = *res.Account.OutboxURI
	}
	typeName := collectionType(c)
	return sendActivityJSON(c, orderedCollectionResponse{
		Context:      "https://www.w3.org/ns/activitystreams",
		ID:           collectionID(c, id),
		Type:         typeName,
		TotalItems:   res.Total,
		First:        firstPage(c, id),
		PartOf:       partOf(c, id),
		OrderedItems: items,
	})
}

func (h *UsersWebHandler) followingCollection(c *fiber.Ctx) error {
	res, derr := h.getFollowing.GetFollowing(c.UserContext(), c.Params("username"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	items := make([]json.RawMessage, 0, len(res.Following))
	for _, follow := range res.Following {
		items = append(items, json.RawMessage(fmt.Sprintf("%q", follow.RemoteActor)))
	}
	return sendActivityJSON(c, orderedCollectionResponse{
		Context:      "https://www.w3.org/ns/activitystreams",
		ID:           res.Account.FollowingURI,
		Type:         "OrderedCollection",
		TotalItems:   len(items),
		OrderedItems: items,
	})
}

func (h *UsersWebHandler) createFollowing(c *fiber.Ctx) error {
	var input struct {
		Actor string `json:"actor"`
		Inbox string `json:"inbox"`
	}
	if err := json.Unmarshal(c.Body(), &input); err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrBadRequest, err))
	}
	if input.Actor == "" {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrBadRequest, "actor is required"))
	}
	if input.Inbox == "" {
		account, derr := h.createFollowingUC.GetLocalAccount(c.UserContext(), c.Params("username"))
		if derr != nil {
			return web.HandleDomainError(c, derr)
		}
		input.Inbox = h.fetchActorInbox(c.UserContext(), input.Actor, account)
	}
	followID, err := dbUtils.NewULID()
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	res, derr := h.createFollowingUC.CreateFollowing(c.UserContext(), apUsecases.CreateFollowingInput{Username: c.Params("username"), Actor: input.Actor, Inbox: input.Inbox, FollowID: followID})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if res.Inbox != "" {
		h.deliverSigned(c.UserContext(), res.RawJSON, res.Inbox, res.Account)
	}
	return c.SendStatus(fiber.StatusCreated)
}

func (h *UsersWebHandler) followersCollection(c *fiber.Ctx) error {
	limit, offset := pagination(c)
	res, derr := h.getFollowers.GetFollowers(c.UserContext(), c.Params("username"), apUsecases.PaginationInput{Limit: limit, Offset: offset})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}

	items := make([]json.RawMessage, 0, len(res.Followers))
	for _, follower := range res.Followers {
		items = append(items, json.RawMessage(fmt.Sprintf("%q", follower.RemoteActor)))
	}

	typeName := collectionType(c)
	return sendActivityJSON(c, orderedCollectionResponse{
		Context:      "https://www.w3.org/ns/activitystreams",
		ID:           collectionID(c, res.Account.FollowersURI),
		Type:         typeName,
		TotalItems:   res.Total,
		First:        firstPage(c, res.Account.FollowersURI),
		PartOf:       partOf(c, res.Account.FollowersURI),
		OrderedItems: items,
	})
}

func (h *UsersWebHandler) handleInboxActivity(c *fiber.Ctx) error {
	raw := append([]byte(nil), c.Body()...)
	account, activity, derr := h.handleInboxActivityUC.InspectInboxActivity(c.UserContext(), c.Params("username"), raw)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.verifyInboundSignature(c, raw, activity.Actor, account, h.cfg.RequireSignedInbox); derr != nil {
		return web.HandleDomainError(c, derr)
	}

	inbox := activity.Inbox
	if activity.Type == "Follow" && inbox == "" {
		inbox = h.fetchActorInbox(c.UserContext(), activity.Actor, account)
	}
	res, derr := h.handleInboxActivityUC.HandleInboxActivity(c.UserContext(), apUsecases.HandleInboxActivityInput{Username: c.Params("username"), RawJSON: raw, Inbox: inbox})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if len(res.AcceptJSON) > 0 && res.AcceptInbox != "" {
		h.deliverSigned(c.UserContext(), res.AcceptJSON, res.AcceptInbox, res.Account)
	}
	return c.SendStatus(fiber.StatusAccepted)
}

func (h *UsersWebHandler) createOutboxActivity(c *fiber.Ctx) error {
	activityID, err := dbUtils.NewULID()
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	objectID, err := dbUtils.NewULID()
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	res, derr := h.createOutboxActivityUC.CreateOutboxActivity(c.UserContext(), apUsecases.CreateOutboxActivityInput{Username: c.Params("username"), RawJSON: append([]byte(nil), c.Body()...), ActivityID: activityID, ObjectID: objectID})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	for _, inbox := range res.FollowerInboxes {
		h.deliverSigned(c.UserContext(), res.RawJSON, inbox, res.Account)
	}
	return c.SendStatus(fiber.StatusCreated)
}

func collectionType(c *fiber.Ctx) string {
	if c.Query("page") != "" {
		return "OrderedCollectionPage"
	}
	return "OrderedCollection"
}

func collectionID(c *fiber.Ctx, base string) string {
	if c.Query("page") == "" {
		return base
	}
	return base + "?page=" + url.QueryEscape(c.Query("page")) + "&limit=" + fmt.Sprint(pageLimit(c))
}

func firstPage(c *fiber.Ctx, base string) string {
	if c.Query("page") != "" {
		return ""
	}
	return base + "?page=1&limit=" + fmt.Sprint(pageLimit(c))
}

func partOf(c *fiber.Ctx, base string) string {
	if c.Query("page") == "" {
		return ""
	}
	return base
}

func pagination(c *fiber.Ctx) (int, int) {
	if c.Query("page") == "" {
		return 0, 0
	}
	limit := pageLimit(c)
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	return limit, (page - 1) * limit
}

func pageLimit(c *fiber.Ctx) int {
	limit := c.QueryInt("limit", 20)
	if limit < 1 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
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

func (h *UsersWebHandler) fetchActorInbox(ctx context.Context, actor string, signer *models.Account) string {
	actorDoc, err := h.fetchActorDocument(ctx, actor, signer)
	if err != nil {
		return ""
	}
	return actorDoc.Inbox
}

func (h *UsersWebHandler) fetchActorDocument(ctx context.Context, actor string, signer *models.Account) (remoteActorDocument, error) {
	if actor == "" {
		return remoteActorDocument{}, errors.New("empty actor")
	}
	client := h.cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actor, nil)
	if err != nil {
		return remoteActorDocument{}, err
	}
	req.Header.Set("Accept", "application/activity+json")
	if signer != nil {
		signOutboundRequest(req, nil, *signer)
	}
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

func (h *UsersWebHandler) deliverSigned(ctx context.Context, body []byte, inbox string, account models.Account) {
	client := h.cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	retries := h.cfg.DeliveryRetries
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

func (h *UsersWebHandler) verifyInboundSignature(c *fiber.Ctx, body []byte, actor string, localAccount *models.Account, required bool) *domainerrors.DomainError {
	sigHeader := c.Get("Signature")
	if sigHeader == "" {
		if required {
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

	if derr := validateHTTPDate(c.Get("Date")); derr != nil {
		return derr
	}

	digest := c.Get("Digest")
	if digest != "" && !strings.EqualFold(digest, digestHeader(body)) {
		return domainerrors.New(domainerrors.ErrUnauthorized, "digest mismatch")
	}

	actorDoc, err := h.fetchActorDocument(c.UserContext(), actor, localAccount)
	if err != nil || actorDoc.PublicKey.PublicKeyPem == "" {
		return domainerrors.New(domainerrors.ErrUnauthorized, "could not fetch actor public key")
	}
	if !validKeyID(params["keyId"], actor, actorDoc) {
		return domainerrors.New(domainerrors.ErrUnauthorized, "signature keyId does not match actor")
	}
	pub, err := parseRSAPublicKey(actorDoc.PublicKey.PublicKeyPem)
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrUnauthorized, err)
	}

	u, _ := url.Parse(c.OriginalURL())
	headerValues := map[string]string{
		"host":         c.Hostname(),
		"date":         c.Get("Date"),
		"digest":       digest,
		"content-type": c.Get("Content-Type"),
	}
	signed := signatureString(c.Method(), u, headerValues, headers)
	hash := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, hash[:], sig); err != nil {
		return domainerrors.NewErr(domainerrors.ErrUnauthorized, err)
	}
	return nil
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

func validKeyID(keyID string, actor string, doc remoteActorDocument) bool {
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
