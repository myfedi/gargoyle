package mastodon

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
	"github.com/myfedi/gargoyle/domain/usecases/oauth"
	infraDB "github.com/myfedi/gargoyle/infrastructure/db"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

// APIHandler exposes the small Mastodon-compatible API surface needed by web
// and mobile clients after OAuth login.
type APIHandler struct {
	host           string
	domain         string
	serverVersion  string
	oauth          oauth.UseCase
	notesRepo      repos.NotesRepository
	createOutboxUC apUsecases.CreateOutboxActivityUseCase
}

type APIHandlerConfig struct {
	Host           string
	Domain         string
	ServerVersion  string
	OAuth          oauth.UseCase
	NotesRepo      repos.NotesRepository
	CreateOutboxUC apUsecases.CreateOutboxActivityUseCase
}

func NewAPIHandler(cfg APIHandlerConfig) APIHandler {
	if cfg.Host == "" || cfg.Domain == "" {
		panic("mastodon API handler requires Host and Domain")
	}
	if cfg.NotesRepo == nil {
		panic("mastodon API handler requires NotesRepo")
	}
	return APIHandler{host: cfg.Host, domain: cfg.Domain, serverVersion: cfg.ServerVersion, oauth: cfg.OAuth, notesRepo: cfg.NotesRepo, createOutboxUC: cfg.CreateOutboxUC}
}

func (h APIHandler) Setup(app *fiber.App) {
	app.Get("/api/v1/instance", h.instanceV1)
	app.Get("/api/v2/instance", h.instanceV2)
	app.Post("/api/v1/statuses", h.createStatus)
	app.Get("/api/v1/timelines/home", h.homeTimeline)
	app.Get("/api/v1/timelines/public", h.publicTimeline)
}

type instanceV1Response struct {
	URI         string `json:"uri"`
	Title       string `json:"title"`
	ShortDesc   string `json:"short_description"`
	Description string `json:"description"`
	Email       string `json:"email"`
	Version     string `json:"version"`
	URLs        struct {
		StreamingAPI string `json:"streaming_api"`
	} `json:"urls"`
	Stats struct {
		UserCount   int `json:"user_count"`
		StatusCount int `json:"status_count"`
		DomainCount int `json:"domain_count"`
	} `json:"stats"`
}

func (h APIHandler) instanceV1(c *fiber.Ctx) error {
	resp := instanceV1Response{URI: h.domain, Title: "Gargoyle", ShortDesc: "Gargoyle federated server", Description: "Gargoyle federated server", Version: h.serverVersion}
	resp.URLs.StreamingAPI = h.host
	return c.JSON(resp)
}

type instanceV2Response struct {
	Domain      string `json:"domain"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	SourceURL   string `json:"source_url"`
	Description string `json:"description"`
}

func (h APIHandler) instanceV2(c *fiber.Ctx) error {
	return c.JSON(instanceV2Response{Domain: h.domain, Title: "Gargoyle", Version: h.serverVersion, Description: "Gargoyle federated server"})
}

type createStatusRequest struct {
	Status     string `json:"status" form:"status"`
	Visibility string `json:"visibility" form:"visibility"`
}

func (h APIHandler) createStatus(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req createStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	if strings.TrimSpace(req.Status) == "" {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrBadRequest, "status is required"))
	}
	activityID, err := infraDB.NewULID()
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	objectID, err := infraDB.NewULID()
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	raw, err := json.Marshal(map[string]any{"type": "Note", "content": req.Status})
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	res, derr := h.createOutboxUC.CreateOutboxActivity(c.UserContext(), apUsecases.CreateOutboxActivityInput{Username: principal.Account.Username, RawJSON: raw, ActivityID: activityID, ObjectID: objectID})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if note, ok := apUsecases.ExtractNote(res.RawJSON); ok {
		return c.JSON(noteToStatus(noteModelFromExtracted(note, principal.Account.ID), principal.Account))
	}
	return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrInternal, "created activity did not contain a note"))
}

func (h APIHandler) homeTimeline(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	notes, err := h.notesRepo.ListLocalNotes(c.UserContext(), nil, principal.Account.ID)
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	return c.JSON(notesToStatuses(notes, principal.Account))
}

func (h APIHandler) publicTimeline(c *fiber.Ctx) error {
	return h.homeTimeline(c)
}

func (h APIHandler) authenticate(c *fiber.Ctx) (*oauth.AuthenticatedUser, *domainerrors.DomainError) {
	auth := c.Get(fiber.HeaderAuthorization)
	bearer := strings.TrimPrefix(auth, "Bearer ")
	return h.oauth.AuthenticateBearer(c.UserContext(), bearer)
}

type statusResponse struct {
	ID               string          `json:"id"`
	URI              string          `json:"uri"`
	URL              string          `json:"url"`
	CreatedAt        string          `json:"created_at"`
	Account          accountResponse `json:"account"`
	Content          string          `json:"content"`
	Visibility       string          `json:"visibility"`
	Sensitive        bool            `json:"sensitive"`
	SpoilerText      string          `json:"spoiler_text"`
	MediaAttachments []any           `json:"media_attachments"`
	Mentions         []any           `json:"mentions"`
	Tags             []any           `json:"tags"`
	Emojis           []any           `json:"emojis"`
	RepliesCount     int             `json:"replies_count"`
	ReblogsCount     int             `json:"reblogs_count"`
	FavouritesCount  int             `json:"favourites_count"`
	Favourited       bool            `json:"favourited"`
	Reblogged        bool            `json:"reblogged"`
	Muted            bool            `json:"muted"`
	Bookmarked       bool            `json:"bookmarked"`
	Pinned           bool            `json:"pinned"`
}

func notesToStatuses(notes []models.Note, account *models.Account) []statusResponse {
	statuses := make([]statusResponse, 0, len(notes))
	for _, note := range notes {
		statuses = append(statuses, noteToStatus(note, account))
	}
	return statuses
}

func noteToStatus(note models.Note, account *models.Account) statusResponse {
	created := note.PublishedAt
	if created.IsZero() {
		created = note.CreatedAt
	}
	if created.IsZero() {
		created = time.Now().UTC()
	}
	return statusResponse{ID: note.ID, URI: note.URI, URL: note.URI, CreatedAt: created.UTC().Format(time.RFC3339), Account: accountToResponse(account), Content: note.Content, Visibility: "public", Sensitive: false, MediaAttachments: []any{}, Mentions: []any{}, Tags: []any{}, Emojis: []any{}}
}

func noteModelFromExtracted(note apUsecases.ExtractedNote, accountID string) models.Note {
	return models.Note{ID: note.URI, LocalAccountID: accountID, URI: note.URI, Content: note.Content, PlainText: note.Content, AttributedTo: note.AttributedTo, PublishedAt: note.PublishedAt}
}
