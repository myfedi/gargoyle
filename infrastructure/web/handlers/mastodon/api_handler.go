package mastodon

import (
	"io"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	mastodonUC "github.com/myfedi/gargoyle/domain/usecases/mastodon"
	"github.com/myfedi/gargoyle/domain/usecases/oauth"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

// APIHandler exposes the small Mastodon-compatible API surface needed by web
// and mobile clients after OAuth login.
type DeliveryQueue func(body []byte, inbox string, account models.Account) *domainerrors.DomainError

type APIHandler struct {
	oauth         oauth.UseCase
	api           mastodonUC.UseCase
	queueDelivery DeliveryQueue
}

type APIHandlerConfig struct {
	OAuth         oauth.UseCase
	API           mastodonUC.UseCase
	QueueDelivery DeliveryQueue
}

func NewAPIHandler(cfg APIHandlerConfig) APIHandler {
	if cfg.QueueDelivery == nil {
		panic("mastodon API handler requires QueueDelivery")
	}
	return APIHandler{oauth: cfg.OAuth, api: cfg.API, queueDelivery: cfg.QueueDelivery}
}

func (h APIHandler) Setup(app *fiber.App) {
	app.Get("/api/v1/instance", h.instanceV1)
	app.Get("/api/v2/instance", h.instanceV2)
	app.Get("/api/v2/search", h.search)
	app.Get("/api/v1/accounts/search", h.search)
	app.Get("/api/v1/accounts/relationships", h.relationships)
	app.Post("/api/v2/media", h.uploadMedia)
	app.Post("/api/v1/media", h.uploadMedia)
	app.Get("/media/:id", h.media)
	app.Get("/api/v1/accounts/:id/followers", h.followers)
	app.Get("/api/v1/accounts/:id/following", h.following)
	app.Get("/api/v1/accounts/:id/statuses", h.accountStatuses)
	app.Post("/api/v1/accounts/:id/follow", h.followAccount)
	app.Post("/api/v1/accounts/:id/unfollow", h.unfollowAccount)
	app.Get("/api/v1/accounts/:id", h.account)
	app.Post("/api/v1/statuses", h.createStatus)
	app.Get("/api/v1/statuses/:id/context", h.statusContext)
	app.Get("/api/v1/statuses/:id", h.status)
	app.Delete("/api/v1/statuses/:id", h.deleteStatus)
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
	info := h.api.InstanceInfo()
	resp := instanceV1Response{URI: info.Domain, Title: info.Title, ShortDesc: info.Description, Description: info.Description, Version: info.ServerVersion}
	resp.URLs.StreamingAPI = info.Host
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
	info := h.api.InstanceInfo()
	return c.JSON(instanceV2Response{Domain: info.Domain, Title: info.Title, Version: info.ServerVersion, Description: info.Description})
}

type mediaAttachmentResponse struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	PreviewURL  string `json:"preview_url"`
	Description string `json:"description"`
}

func (h APIHandler) uploadMedia(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	file, err := c.FormFile("file")
	if err != nil {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrBadRequest, "media file is required"))
	}
	opened, err := file.Open()
	if err != nil {
		return err
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, 10<<20))
	if err != nil {
		return err
	}
	media, derr := h.api.UploadMedia(c.UserContext(), principal.Account, mastodonUC.UploadMediaInput{FileName: file.Filename, ContentType: file.Header.Get("Content-Type"), Data: data, Description: c.FormValue("description")})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(h.mediaResponse(media))
}

func (h APIHandler) media(c *fiber.Ctx) error {
	media, derr := h.api.GetMedia(c.UserContext(), c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	c.Set(fiber.HeaderContentType, media.ContentType)
	return c.Send(media.Data)
}

func (h APIHandler) mediaResponse(media *models.MediaAttachment) mediaAttachmentResponse {
	url := "/media/" + media.ID
	return mediaAttachmentResponse{ID: media.ID, Type: "image", URL: url, PreviewURL: url, Description: media.Description}
}

type createStatusRequest struct {
	Status      string `json:"status" form:"status"`
	Visibility  string `json:"visibility" form:"visibility"`
	InReplyToID string `json:"in_reply_to_id" form:"in_reply_to_id"`
	Sensitive   bool   `json:"sensitive" form:"sensitive"`
	SpoilerText string `json:"spoiler_text" form:"spoiler_text"`
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
	res, derr := h.api.CreateStatus(c.UserContext(), principal.Account, mastodonUC.CreateStatusInput{Content: req.Status, Visibility: req.Visibility, InReplyToID: req.InReplyToID, Sensitive: req.Sensitive, SpoilerText: req.SpoilerText})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	for _, inbox := range res.FollowerInboxes {
		if err := h.queueDelivery(res.RawJSON, inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(noteToStatus(res.Note, principal.Account))
}

type searchResponse struct {
	Accounts []accountResponse `json:"accounts"`
	Statuses []statusResponse  `json:"statuses"`
	Hashtags []any             `json:"hashtags"`
}

func (h APIHandler) search(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	accounts, derr := h.api.SearchAccounts(c.UserContext(), principal.Account, c.Query("q"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	resp := searchResponse{Accounts: make([]accountResponse, 0, len(accounts)), Statuses: []statusResponse{}, Hashtags: []any{}}
	for _, account := range accounts {
		acct := accountToResponse(&account)
		if account.Domain != nil && *account.Domain != "" {
			acct.Acct = account.Username + "@" + *account.Domain
		}
		resp.Accounts = append(resp.Accounts, acct)
	}
	return c.JSON(resp)
}

type relationshipResponse struct {
	ID                  string `json:"id"`
	Following           bool   `json:"following"`
	ShowingReblogs      bool   `json:"showing_reblogs"`
	Notifying           bool   `json:"notifying"`
	FollowedBy          bool   `json:"followed_by"`
	Blocking            bool   `json:"blocking"`
	BlockedBy           bool   `json:"blocked_by"`
	Muting              bool   `json:"muting"`
	MutingNotifications bool   `json:"muting_notifications"`
	Requested           bool   `json:"requested"`
	DomainBlocking      bool   `json:"domain_blocking"`
	Endorsed            bool   `json:"endorsed"`
}

func (h APIHandler) relationships(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	ids := c.Queries()["id[]"]
	if ids == "" {
		ids = c.Query("id")
	}
	idList := strings.Split(ids, ",")
	relationships, derr := h.api.Relationships(c.UserContext(), principal.Account, idList)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	resp := make([]relationshipResponse, 0, len(idList))
	for _, id := range idList {
		if id == "" {
			continue
		}
		rel := relationships[id]
		resp = append(resp, relationshipResponse{ID: id, Following: rel.Following, Requested: rel.Requested, ShowingReblogs: true})
	}
	return c.JSON(resp)
}

func (h APIHandler) account(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	account, derr := h.api.GetAccount(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(accountToResponse(account))
}

func (h APIHandler) accountList(c *fiber.Ctx, accounts []models.Account) error {
	resp := make([]accountResponse, 0, len(accounts))
	for _, account := range accounts {
		acct := accountToResponse(&account)
		if account.Domain != nil && *account.Domain != "" {
			acct.Acct = account.Username + "@" + *account.Domain
		}
		resp = append(resp, acct)
	}
	return c.JSON(resp)
}

func (h APIHandler) accountStatuses(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	items, derr := h.api.AccountStatuses(c.UserContext(), principal.Account, c.Params("id"), c.QueryInt("limit"), c.Query("max_id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(timelineItemsToStatuses(items))
}

func (h APIHandler) followers(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	accounts, derr := h.api.FollowerAccounts(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return h.accountList(c, accounts)
}

func (h APIHandler) following(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	accounts, derr := h.api.FollowingAccounts(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return h.accountList(c, accounts)
}

func (h APIHandler) followAccount(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	res, derr := h.api.FollowAccount(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if res.Inbox != "" {
		if err := h.queueDelivery(res.RawJSON, res.Inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(relationshipResponse{ID: c.Params("id"), Following: false, Requested: true, ShowingReblogs: true})
}

func (h APIHandler) unfollowAccount(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	res, derr := h.api.UnfollowAccount(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if res.Inbox != "" {
		if err := h.queueDelivery(res.RawJSON, res.Inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(relationshipResponse{ID: c.Params("id"), Following: false, Requested: false, ShowingReblogs: true})
}

func (h APIHandler) status(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	item, derr := h.api.GetStatus(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	statuses := timelineItemsToStatuses([]mastodonUC.TimelineItem{*item})
	return c.JSON(statuses[0])
}

func (h APIHandler) deleteStatus(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	res, derr := h.api.DeleteStatus(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	for _, inbox := range res.FollowerInboxes {
		if err := h.queueDelivery(res.RawJSON, inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.SendStatus(fiber.StatusOK)
}

type contextResponse struct {
	Ancestors   []statusResponse `json:"ancestors"`
	Descendants []statusResponse `json:"descendants"`
}

func (h APIHandler) statusContext(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	ctxResp, derr := h.api.StatusContext(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(contextResponse{Ancestors: timelineItemsToStatuses(ctxResp.Ancestors), Descendants: timelineItemsToStatuses(ctxResp.Descendants)})
}

func (h APIHandler) homeTimeline(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	items, derr := h.api.HomeTimeline(c.UserContext(), principal.Account, timelineOptions(c))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(timelineItemsToStatuses(items))
}

func (h APIHandler) publicTimeline(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	items, derr := h.api.PublicTimeline(c.UserContext(), principal.Account, timelineOptions(c))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(timelineItemsToStatuses(items))
}

func (h APIHandler) authenticate(c *fiber.Ctx) (*oauth.AuthenticatedUser, *domainerrors.DomainError) {
	auth := c.Get(fiber.HeaderAuthorization)
	bearer := strings.TrimPrefix(auth, "Bearer ")
	return h.oauth.AuthenticateBearer(c.UserContext(), bearer)
}

type statusResponse struct {
	ID                 string                    `json:"id"`
	URI                string                    `json:"uri"`
	URL                string                    `json:"url"`
	CreatedAt          string                    `json:"created_at"`
	Account            accountResponse           `json:"account"`
	Content            string                    `json:"content"`
	Visibility         string                    `json:"visibility"`
	InReplyToID        *string                   `json:"in_reply_to_id"`
	InReplyToAccountID *string                   `json:"in_reply_to_account_id"`
	Sensitive          bool                      `json:"sensitive"`
	SpoilerText        string                    `json:"spoiler_text"`
	MediaAttachments   []mediaAttachmentResponse `json:"media_attachments"`
	Mentions           []any                     `json:"mentions"`
	Tags               []any                     `json:"tags"`
	Emojis             []any                     `json:"emojis"`
	RepliesCount       int                       `json:"replies_count"`
	ReblogsCount       int                       `json:"reblogs_count"`
	FavouritesCount    int                       `json:"favourites_count"`
	Favourited         bool                      `json:"favourited"`
	Reblogged          bool                      `json:"reblogged"`
	Muted              bool                      `json:"muted"`
	Bookmarked         bool                      `json:"bookmarked"`
	Pinned             bool                      `json:"pinned"`
}

func timelineOptions(c *fiber.Ctx) mastodonUC.TimelineOptions {
	return mastodonUC.TimelineOptions{Limit: c.QueryInt("limit"), MaxID: c.Query("max_id"), LocalOnly: c.QueryBool("local"), RemoteOnly: c.QueryBool("remote")}
}

func timelineItemsToStatuses(items []mastodonUC.TimelineItem) []statusResponse {
	statuses := make([]statusResponse, 0, len(items))
	for _, item := range items {
		status := noteToStatus(item.Note, &item.Account)
		status.InReplyToAccountID = item.InReplyToAccountID
		statuses = append(statuses, status)
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
	visibility := note.Visibility
	if visibility == "" {
		visibility = "public"
	}
	return statusResponse{ID: note.ID, URI: note.URI, URL: note.URI, CreatedAt: created.UTC().Format(time.RFC3339), Account: accountToResponse(account), Content: note.Content, Visibility: visibility, InReplyToID: note.InReplyToID, Sensitive: note.Sensitive, SpoilerText: note.SpoilerText, MediaAttachments: []mediaAttachmentResponse{}, Mentions: []any{}, Tags: []any{}, Emojis: []any{}}
}
