package users

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/activitypub"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

// UsersWebHandlerConfig wires HTTP infrastructure to ActivityPub use cases.
// Required dependencies are validated in NewUsersWebHandler so configuration
// mistakes fail at startup instead of during request handling.
type UsersWebHandlerConfig struct {
	TxProvider          db.TxProvider
	AccountsRepo        repos.AccountsRepo
	ActivitiesRepo      repos.ActivitiesRepository
	FollowsRepo         repos.FollowsRepository
	NotesRepo           repos.NotesRepository
	DeliveryJobsRepo    repos.DeliveryJobsRepository
	Serializer          activitypub.ActorSerializer
	ActorFetcher        activitypub.ActorFetcher
	Deliverer           activitypub.ActivityDeliverer
	SignatureVerifier   activitypub.SignatureVerifier
	ContentSanitizer    ports.ContentSanitizer
	HTTPClient          *http.Client
	BodyLimitBytes      int
	RemoteURLExceptions []RemoteURLException
	RequireSignedInbox  bool
	AllowUnsignedInbox  bool
	DeliveryRetries     int
}

type UsersWebHandler struct {
	cfg                   UsersWebHandlerConfig
	handler               apUsecases.GetUserProfileUseCase
	getOutbox             apUsecases.GetOutboxUseCase
	getFollowers          apUsecases.GetFollowersUseCase
	getFollowing          apUsecases.GetFollowingUseCase
	handleInboxActivityUC apUsecases.HandleInboxActivityUseCase
}

// NewWebfingerWebHandler creates a new Webfinger handler with the given dependencies.

func validateUsersWebHandlerConfig(cfg UsersWebHandlerConfig) {
	if cfg.TxProvider == nil {
		panic("users web handler requires TxProvider")
	}
	if cfg.AccountsRepo == nil {
		panic("users web handler requires AccountsRepo")
	}
	if cfg.ActivitiesRepo == nil {
		panic("users web handler requires ActivitiesRepo")
	}
	if cfg.FollowsRepo == nil {
		panic("users web handler requires FollowsRepo")
	}
	if cfg.NotesRepo == nil {
		panic("users web handler requires NotesRepo")
	}
	if cfg.DeliveryJobsRepo == nil {
		panic("users web handler requires DeliveryJobsRepo")
	}
	if cfg.Serializer == nil {
		panic("users web handler requires Serializer")
	}
	if !cfg.RequireSignedInbox && !cfg.AllowUnsignedInbox {
		panic("users web handler requires signed inbox unless AllowUnsignedInbox is explicitly enabled")
	}
	if cfg.ContentSanitizer == nil {
		panic("users web handler requires ContentSanitizer")
	}
	if cfg.BodyLimitBytes <= 0 {
		panic("users web handler requires positive BodyLimitBytes")
	}
}

func ensureBodySize(body []byte, limit int) *domainerrors.DomainError {
	if len(body) > limit {
		return domainerrors.New(domainerrors.ErrBadRequest, "request body too large")
	}
	return nil
}

// ActivityDeliverer exposes the configured delivery adapter to process-level
// workers without making other handlers know about HTTP-signature transport.
func (h *UsersWebHandler) ActivityDeliverer() activitypub.ActivityDeliverer {
	return h.cfg.Deliverer
}

// QueueDelivery enqueues a committed ActivityPub payload for asynchronous
// delivery. Other HTTP adapters use this to keep network I/O out of use cases
// while preserving the shared delivery worker and backpressure behavior.
func (h *UsersWebHandler) QueueDelivery(body []byte, inbox string, account models.Account) *domainerrors.DomainError {
	return h.queueDelivery(body, inbox, account)
}

func (h *UsersWebHandler) queueDelivery(body []byte, inbox string, account models.Account) *domainerrors.DomainError {
	_, err := h.cfg.DeliveryJobsRepo.CreateDeliveryJob(context.Background(), nil, repos.CreateDeliveryJobInput{AccountID: account.ID, InboxURL: inbox, Payload: append([]byte(nil), body...), NextAttemptAt: time.Now().UTC()})
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return nil
}

func NewUsersWebHandler(cfg UsersWebHandlerConfig) *UsersWebHandler {
	validateUsersWebHandlerConfig(cfg)
	handler := apUsecases.NewGetUserProfileUseCase(apUsecases.GetUserProfileUseCaseConfig{
		AccountsRepo: cfg.AccountsRepo,
		Serializer:   cfg.Serializer,
	})
	transport := httpActivityPubTransport{client: cfg.HTTPClient, retries: cfg.DeliveryRetries, exceptions: cfg.RemoteURLExceptions}
	if cfg.ActorFetcher == nil {
		cfg.ActorFetcher = transport
	}
	if cfg.Deliverer == nil {
		cfg.Deliverer = transport
	}
	if cfg.SignatureVerifier == nil {
		cfg.SignatureVerifier = transport
	}
	flowCfg := apUsecases.ActivityPubFlowConfig{
		TxProvider:       cfg.TxProvider,
		AccountsRepo:     cfg.AccountsRepo,
		ActivitiesRepo:   cfg.ActivitiesRepo,
		FollowsRepo:      cfg.FollowsRepo,
		NotesRepo:        cfg.NotesRepo,
		ActorFetcher:     cfg.ActorFetcher,
		ContentSanitizer: cfg.ContentSanitizer,
	}
	h := &UsersWebHandler{
		cfg:                   cfg,
		handler:               handler,
		getOutbox:             apUsecases.NewGetOutboxUseCase(flowCfg),
		getFollowers:          apUsecases.NewGetFollowersUseCase(flowCfg),
		getFollowing:          apUsecases.NewGetFollowingUseCase(flowCfg),
		handleInboxActivityUC: apUsecases.NewHandleInboxActivityUseCase(flowCfg),
	}
	return h
}

// SetupHostMeta initializes the hostmeta route for the Fiber application.
func (h *UsersWebHandler) SetupUserProfileHandler(app *fiber.App) {
	app.Get("/@:username", func(c *fiber.Ctx) error {
		return c.Redirect("/users/"+url.PathEscape(c.Params("username")), fiber.StatusFound)
	})

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
	app.Get("/users/:username/followers", h.followersCollection)
	app.Get("/users/:username/following", h.followingCollection)

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
	if err := ensureBodySize(c.Body(), h.cfg.BodyLimitBytes); err != nil {
		return web.HandleDomainError(c, err)
	}
	raw := append([]byte(nil), c.Body()...)
	account, activity, derr := h.handleInboxActivityUC.InspectInboxActivity(c.UserContext(), c.Params("username"), raw)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.cfg.SignatureVerifier.VerifyInbound(c.UserContext(), signatureVerificationInput(c, raw, activity.Actor, account, h.cfg.RequireSignedInbox)); derr != nil {
		return web.HandleDomainError(c, derr)
	}

	res, derr := h.handleInboxActivityUC.HandleInboxActivity(c.UserContext(), apUsecases.HandleInboxActivityInput{Username: c.Params("username"), RawJSON: raw, Inbox: activity.Inbox})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if len(res.AcceptJSON) > 0 && res.AcceptInbox != "" {
		if err := h.queueDelivery(res.AcceptJSON, res.AcceptInbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.SendStatus(fiber.StatusAccepted)
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
