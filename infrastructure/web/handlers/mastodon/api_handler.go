package mastodon

import (
	"context"
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
	app.Get("/api/v1/accounts/search", h.accountsSearch)
	app.Get("/api/v1/accounts/relationships", h.relationships)
	app.Get("/api/v1/notifications", h.notifications)
	app.Post("/api/v1/notifications/clear", h.clearNotifications)
	app.Post("/api/v1/notifications/:id/dismiss", h.dismissNotification)
	app.Delete("/api/v1/notifications/:id", h.dismissNotification)
	app.Get("/api/v1/favourites", h.favouriteStatuses)
	app.Get("/api/v1/bookmarks", h.bookmarkedStatuses)
	app.Get("/api/v1/preferences", h.preferences)
	app.Get("/api/v1/conversations", h.conversations)
	app.Delete("/api/v1/conversations/:id", h.deleteConversation)
	app.Post("/api/v1/conversations/:id/read", h.readConversation)
	app.Get("/api/v1/custom_emojis", h.customEmojis)
	app.Get("/api/v1/announcements", h.emptyList)
	app.Get("/api/v1/trends/tags", h.emptyList)
	app.Get("/api/v1/trends/statuses", h.emptyList)
	app.Get("/api/v1/trends/links", h.emptyList)
	app.Get("/api/v1/lists", h.emptyList)
	app.Get("/api/v1/filters", h.emptyList)
	app.Get("/api/v2/filters", h.emptyList)
	app.Post("/api/v2/media", h.uploadMedia)
	app.Post("/api/v1/media", h.uploadMedia)
	app.Get("/api/v1/media/:id", h.getMediaAttachment)
	app.Put("/api/v1/media/:id", h.updateMedia)
	app.Delete("/api/v1/media/:id", h.deleteMedia)
	app.Get("/media/:id", h.media)
	app.Get("/media/:id/:filename", h.media)
	app.Head("/media/:id", h.media)
	app.Head("/media/:id/:filename", h.media)
	app.Get("/api/v1/accounts/:id/followers", h.followers)
	app.Get("/api/v1/accounts/:id/following", h.following)
	app.Get("/api/v1/accounts/:id/statuses", h.accountStatuses)
	app.Patch("/api/v1/accounts/update_credentials", h.updateCredentials)
	app.Post("/api/v1/accounts/:id/follow", h.followAccount)
	app.Post("/api/v1/accounts/:id/unfollow", h.unfollowAccount)
	app.Get("/api/v1/accounts/:id", h.account)
	app.Post("/api/v1/statuses", h.createStatus)
	app.Put("/api/v1/statuses/:id", h.updateStatus)
	app.Patch("/api/v1/statuses/:id", h.updateStatus)
	app.Get("/api/v1/statuses/:id/source", h.statusSource)
	app.Get("/api/v1/statuses/:id/history", h.statusHistory)
	app.Get("/api/v1/statuses/:id/context", h.statusContext)
	app.Post("/api/v1/statuses/:id/favourite", h.favouriteStatus)
	app.Post("/api/v1/statuses/:id/unfavourite", h.unfavouriteStatus)
	app.Post("/api/v1/statuses/:id/bookmark", h.bookmarkStatus)
	app.Post("/api/v1/statuses/:id/unbookmark", h.unbookmarkStatus)
	app.Post("/api/v1/statuses/:id/pin", h.pinStatus)
	app.Post("/api/v1/statuses/:id/unpin", h.unpinStatus)
	app.Post("/api/v1/statuses/:id/reblog", h.reblogStatus)
	app.Post("/api/v1/statuses/:id/unreblog", h.unreblogStatus)
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
	principal, derr := h.authenticate(c, "write")
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
	data, err := io.ReadAll(io.LimitReader(opened, mastodonUC.MaxMediaUploadBytes+1))
	if err != nil {
		return err
	}
	if len(data) > mastodonUC.MaxMediaUploadBytes {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrBadRequest, "media file is too large"))
	}
	media, derr := h.api.UploadMedia(c.UserContext(), principal.Account, mastodonUC.UploadMediaInput{FileName: file.Filename, ContentType: file.Header.Get("Content-Type"), Data: data, Description: c.FormValue("description")})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(h.mediaResponse(media))
}

type updateMediaRequest struct {
	Description string `json:"description" form:"description"`
}

func (h APIHandler) getMediaAttachment(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	media, derr := h.api.GetMedia(c.UserContext(), c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if media.LocalAccountID != principal.Account.ID {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrNotFound, "media not found"))
	}
	return c.JSON(h.mediaResponse(media))
}

func (h APIHandler) updateMedia(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req updateMediaRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	media, derr := h.api.UpdateMedia(c.UserContext(), principal.Account, c.Params("id"), mastodonUC.UpdateMediaInput{Description: req.Description})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(h.mediaResponse(media))
}

func (h APIHandler) deleteMedia(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.api.DeleteMedia(c.UserContext(), principal.Account, c.Params("id")); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(map[string]any{})
}

func (h APIHandler) media(c *fiber.Ctx) error {
	media, derr := h.api.GetMedia(c.UserContext(), c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	c.Set(fiber.HeaderContentType, media.ContentType)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	return c.Send(media.Data)
}

func (h APIHandler) mediaResponse(media *models.MediaAttachment) mediaAttachmentResponse {
	return mediaResponse(*media)
}

func mediaResponses(media []models.MediaAttachment) []mediaAttachmentResponse {
	res := make([]mediaAttachmentResponse, 0, len(media))
	for _, item := range media {
		res = append(res, mediaResponse(item))
	}
	return res
}

func mediaResponse(media models.MediaAttachment) mediaAttachmentResponse {
	url := "/media/" + media.ID
	return mediaAttachmentResponse{ID: media.ID, Type: mediaType(media.ContentType), URL: url, PreviewURL: url, Description: media.Description}
}

func mediaType(contentType string) string {
	if strings.HasPrefix(contentType, "video/") {
		return "video"
	}
	if strings.HasPrefix(contentType, "audio/") {
		return "audio"
	}
	return "image"
}

func uniqueInboxes(inboxes []string) []string {
	res := make([]string, 0, len(inboxes))
	seen := map[string]bool{}
	for _, inbox := range inboxes {
		if inbox == "" || seen[inbox] {
			continue
		}
		seen[inbox] = true
		res = append(res, inbox)
	}
	return res
}

type createStatusRequest struct {
	Status      string   `json:"status" form:"status"`
	Visibility  string   `json:"visibility" form:"visibility"`
	InReplyToID string   `json:"in_reply_to_id" form:"in_reply_to_id"`
	Sensitive   bool     `json:"sensitive" form:"sensitive"`
	SpoilerText string   `json:"spoiler_text" form:"spoiler_text"`
	MediaIDs    []string `json:"media_ids" form:"media_ids"`
}

type updateStatusRequest struct {
	Status      string   `json:"status" form:"status"`
	Visibility  string   `json:"visibility" form:"visibility"`
	Sensitive   bool     `json:"sensitive" form:"sensitive"`
	SpoilerText string   `json:"spoiler_text" form:"spoiler_text"`
	MediaIDs    []string `json:"media_ids" form:"media_ids"`
}

type updateCredentialsRequest struct {
	DisplayName string `json:"display_name" form:"display_name"`
	Note        string `json:"note" form:"note"`
}

func (h APIHandler) updateCredentials(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req updateCredentialsRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	input := mastodonUC.UpdateCredentialsInput{DisplayName: req.DisplayName, Note: req.Note}
	if avatar, derr := profileUploadFromForm(c, "avatar"); derr != nil {
		return web.HandleDomainError(c, derr)
	} else {
		input.Avatar = avatar
	}
	if header, derr := profileUploadFromForm(c, "header"); derr != nil {
		return web.HandleDomainError(c, derr)
	} else {
		input.Header = header
	}
	res, derr := h.api.UpdateCredentials(c.UserContext(), principal.Account, input)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	for _, inbox := range uniqueInboxes(res.FollowerInboxes) {
		if err := h.queueDelivery(res.RawJSON, inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(accountToResponse(&res.Account))
}

func profileUploadFromForm(c *fiber.Ctx, field string) (*mastodonUC.UploadMediaInput, *domainerrors.DomainError) {
	file, err := c.FormFile(field)
	if err != nil {
		return nil, nil
	}
	opened, err := file.Open()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, mastodonUC.MaxMediaUploadBytes+1))
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if len(data) > mastodonUC.MaxMediaUploadBytes {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "profile image is too large")
	}
	return &mastodonUC.UploadMediaInput{FileName: file.Filename, ContentType: file.Header.Get("Content-Type"), Data: data}, nil
}

func (h APIHandler) createStatus(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req createStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	res, derr := h.api.CreateStatus(c.UserContext(), principal.Account, mastodonUC.CreateStatusInput{Content: req.Status, Visibility: req.Visibility, InReplyToID: req.InReplyToID, Sensitive: req.Sensitive, SpoilerText: req.SpoilerText, MediaIDs: req.MediaIDs})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	for _, inbox := range uniqueInboxes(append(res.FollowerInboxes, res.MentionInboxes...)) {
		if err := h.queueDelivery(res.RawJSON, inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	status := timelineItemsToStatuses([]mastodonUC.TimelineItem{{ID: res.Note.ID, URI: res.Note.URI, CreatedAt: res.Note.PublishedAt, Note: res.Note, Account: res.Account, Media: res.Media, Mentions: res.Mentions}})[0]
	return c.JSON(status)
}

func (h APIHandler) updateStatus(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req updateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	res, derr := h.api.UpdateStatus(c.UserContext(), principal.Account, c.Params("id"), mastodonUC.UpdateStatusInput{Content: req.Status, Visibility: req.Visibility, Sensitive: req.Sensitive, SpoilerText: req.SpoilerText, MediaIDs: req.MediaIDs})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	for _, inbox := range uniqueInboxes(append(res.FollowerInboxes, res.MentionInboxes...)) {
		if err := h.queueDelivery(res.RawJSON, inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	status := timelineItemsToStatuses([]mastodonUC.TimelineItem{{ID: res.Note.ID, URI: res.Note.URI, CreatedAt: res.Note.PublishedAt, Note: res.Note, Account: res.Account, Media: res.Media, Mentions: res.Mentions}})[0]
	return c.JSON(status)
}

type searchResponse struct {
	Accounts []accountResponse `json:"accounts"`
	Statuses []statusResponse  `json:"statuses"`
	Hashtags []any             `json:"hashtags"`
}

func (h APIHandler) accountsSearch(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	accounts, derr := h.api.SearchAccounts(c.UserContext(), principal.Account, c.Query("q"), c.QueryInt("limit"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(accountsToResponses(accounts))
}

func (h APIHandler) search(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var accounts []models.Account
	if c.Query("type") == "" || c.Query("type") == "accounts" {
		if c.QueryBool("resolve") {
			accounts, derr = h.api.ResolveAccountSearch(c.UserContext(), principal.Account, c.Query("q"))
		} else {
			accounts, derr = h.api.SearchAccounts(c.UserContext(), principal.Account, c.Query("q"), c.QueryInt("limit"))
		}
		if derr != nil {
			return web.HandleDomainError(c, derr)
		}
	}
	return c.JSON(searchResponse{Accounts: accountsToResponses(accounts), Statuses: []statusResponse{}, Hashtags: []any{}})
}

func accountsToResponses(accounts []models.Account) []accountResponse {
	resp := make([]accountResponse, 0, len(accounts))
	for _, account := range accounts {
		acct := accountToResponse(&account)
		if account.Domain != nil && *account.Domain != "" {
			acct.Acct = account.Username + "@" + *account.Domain
		}
		resp = append(resp, acct)
	}
	return resp
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
	if c.QueryBool("pinned") {
		items, derr := h.api.PinnedAccountStatuses(c.UserContext(), principal.Account, c.Params("id"), c.QueryInt("limit"))
		if derr != nil {
			return web.HandleDomainError(c, derr)
		}
		return c.JSON(timelineItemsToStatuses(items))
	}
	items, derr := h.api.AccountStatuses(c.UserContext(), principal.Account, c.Params("id"), c.QueryInt("limit"), c.Query("max_id"), c.QueryBool("exclude_reblogs"))
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
	principal, derr := h.authenticate(c, "follow")
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
	principal, derr := h.authenticate(c, "follow")
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

type notificationResponse struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	CreatedAt string          `json:"created_at"`
	Account   accountResponse `json:"account"`
	Status    *statusResponse `json:"status,omitempty"`
}

func (h APIHandler) notifications(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	items, derr := h.api.Notifications(c.UserContext(), principal.Account, c.QueryInt("limit"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	resp := make([]notificationResponse, 0, len(items))
	for _, item := range items {
		var status *statusResponse
		if item.Status != nil {
			s := timelineItemsToStatuses([]mastodonUC.TimelineItem{*item.Status})[0]
			status = &s
		}
		resp = append(resp, notificationResponse{ID: item.Notification.ID, Type: item.Notification.Type, CreatedAt: item.Notification.CreatedAt.UTC().Format(time.RFC3339), Account: accountToResponse(&item.Account), Status: status})
	}
	return c.JSON(resp)
}

func (h APIHandler) clearNotifications(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.api.ClearNotifications(c.UserContext(), principal.Account); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(map[string]any{})
}

func (h APIHandler) dismissNotification(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.api.DismissNotification(c.UserContext(), principal.Account, c.Params("id")); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(map[string]any{})
}

type conversationResponse struct {
	ID         string            `json:"id"`
	Unread     bool              `json:"unread"`
	Accounts   []accountResponse `json:"accounts"`
	LastStatus statusResponse    `json:"last_status"`
}

func (h APIHandler) conversations(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	items, derr := h.api.Conversations(c.UserContext(), principal.Account, c.QueryInt("limit"), c.Query("max_id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	resp := make([]conversationResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, conversationResponse{ID: item.ID, Unread: item.Unread, Accounts: accountsToResponses(item.Accounts), LastStatus: timelineItemsToStatuses([]mastodonUC.TimelineItem{item.LastStatus})[0]})
	}
	return c.JSON(resp)
}

func (h APIHandler) deleteConversation(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.api.DismissConversation(c.UserContext(), principal.Account, c.Params("id")); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(map[string]any{})
}

func (h APIHandler) readConversation(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.api.ReadConversation(c.UserContext(), principal.Account, c.Params("id")); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(map[string]any{})
}

func (h APIHandler) favouriteStatuses(c *fiber.Ctx) error {
	return h.interactionTimeline(c, h.api.FavouriteStatuses)
}

func (h APIHandler) bookmarkedStatuses(c *fiber.Ctx) error {
	return h.interactionTimeline(c, h.api.BookmarkedStatuses)
}

func (h APIHandler) interactionTimeline(c *fiber.Ctx, fn func(context.Context, *models.Account, int) ([]mastodonUC.TimelineItem, *domainerrors.DomainError)) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	items, derr := fn(c.UserContext(), principal.Account, c.QueryInt("limit"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(timelineItemsToStatuses(items))
}

func (h APIHandler) preferences(c *fiber.Ctx) error {
	if _, derr := h.authenticate(c); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(map[string]any{"posting:default:visibility": "public", "posting:default:sensitive": false, "posting:default:language": nil, "reading:expand:media": "default", "reading:expand:spoilers": false})
}

func (h APIHandler) customEmojis(c *fiber.Ctx) error { return h.emptyList(c) }

func (h APIHandler) emptyList(c *fiber.Ctx) error {
	if _, derr := h.authenticate(c); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON([]any{})
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

type statusSourceResponse struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	SpoilerText string `json:"spoiler_text"`
}

type statusHistoryResponse struct {
	Content          string                    `json:"content"`
	SpoilerText      string                    `json:"spoiler_text"`
	Sensitive        bool                      `json:"sensitive"`
	CreatedAt        string                    `json:"created_at"`
	Account          accountResponse           `json:"account"`
	MediaAttachments []mediaAttachmentResponse `json:"media_attachments"`
	Emojis           []any                     `json:"emojis"`
}

func (h APIHandler) statusSource(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	item, derr := h.api.GetStatus(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(statusSourceResponse{ID: item.Note.ID, Text: item.Note.PlainText, SpoilerText: item.Note.SpoilerText})
}

func (h APIHandler) statusHistory(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	history, derr := h.api.StatusHistory(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	resp := make([]statusHistoryResponse, 0, len(history))
	for _, item := range history {
		resp = append(resp, statusHistoryResponse{Content: item.Content, SpoilerText: item.SpoilerText, Sensitive: item.Sensitive, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339), Account: accountToResponse(&item.Account), MediaAttachments: mediaResponses(item.Media), Emojis: []any{}})
	}
	return c.JSON(resp)
}

func (h APIHandler) favouriteStatus(c *fiber.Ctx) error {
	return h.interactStatus(c, h.api.FavouriteStatus)
}
func (h APIHandler) unfavouriteStatus(c *fiber.Ctx) error {
	return h.interactStatus(c, h.api.UnfavouriteStatus)
}
func (h APIHandler) bookmarkStatus(c *fiber.Ctx) error {
	return h.localStatusInteraction(c, h.api.BookmarkStatus)
}
func (h APIHandler) unbookmarkStatus(c *fiber.Ctx) error {
	return h.localStatusInteraction(c, h.api.UnbookmarkStatus)
}
func (h APIHandler) pinStatus(c *fiber.Ctx) error {
	return h.localStatusInteraction(c, h.api.PinStatus)
}
func (h APIHandler) unpinStatus(c *fiber.Ctx) error {
	return h.localStatusInteraction(c, h.api.UnpinStatus)
}
func (h APIHandler) reblogStatus(c *fiber.Ctx) error { return h.interactStatus(c, h.api.ReblogStatus) }
func (h APIHandler) unreblogStatus(c *fiber.Ctx) error {
	return h.interactStatus(c, h.api.UnreblogStatus)
}

func (h APIHandler) localStatusInteraction(c *fiber.Ctx, fn func(context.Context, *models.Account, string) (*mastodonUC.TimelineItem, *domainerrors.DomainError)) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	item, derr := fn(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(timelineItemsToStatuses([]mastodonUC.TimelineItem{*item})[0])
}

func (h APIHandler) interactStatus(c *fiber.Ctx, fn func(context.Context, *models.Account, string) (*mastodonUC.InteractionResult, *domainerrors.DomainError)) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	res, derr := fn(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if res.Delivery != nil && res.Delivery.Inbox != "" {
		if err := h.queueDelivery(res.Delivery.RawJSON, res.Delivery.Inbox, res.Delivery.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(timelineItemsToStatuses([]mastodonUC.TimelineItem{res.Status})[0])
}

func (h APIHandler) deleteStatus(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
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

func (h APIHandler) authenticate(c *fiber.Ctx, requiredScopes ...string) (*oauth.AuthenticatedUser, *domainerrors.DomainError) {
	auth := c.Get(fiber.HeaderAuthorization)
	bearer := strings.TrimPrefix(auth, "Bearer ")
	principal, derr := h.oauth.AuthenticateBearer(c.UserContext(), bearer)
	if derr != nil {
		return nil, derr
	}
	for _, required := range requiredScopes {
		if !scopeIncludes(principal.Scopes, required) {
			return nil, domainerrors.New(domainerrors.ErrUnauthorized, "insufficient OAuth scope")
		}
	}
	return principal, nil
}

func scopeIncludes(scopes string, required string) bool {
	for _, scope := range strings.Fields(scopes) {
		if scope == required {
			return true
		}
		if required == "write" && strings.HasPrefix(scope, "write:") {
			return true
		}
		if required == "read" && strings.HasPrefix(scope, "read:") {
			return true
		}
	}
	return false
}

type mentionResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Acct     string `json:"acct"`
	URL      string `json:"url"`
}

type statusResponse struct {
	ID                 string                    `json:"id"`
	URI                string                    `json:"uri"`
	URL                string                    `json:"url"`
	CreatedAt          string                    `json:"created_at"`
	EditedAt           *string                   `json:"edited_at"`
	Account            accountResponse           `json:"account"`
	Content            string                    `json:"content"`
	Visibility         string                    `json:"visibility"`
	InReplyToID        *string                   `json:"in_reply_to_id"`
	InReplyToAccountID *string                   `json:"in_reply_to_account_id"`
	Sensitive          bool                      `json:"sensitive"`
	SpoilerText        string                    `json:"spoiler_text"`
	MediaAttachments   []mediaAttachmentResponse `json:"media_attachments"`
	Mentions           []mentionResponse         `json:"mentions"`
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
	Reblog             *statusResponse           `json:"reblog"`
}

func timelineOptions(c *fiber.Ctx) mastodonUC.TimelineOptions {
	return mastodonUC.TimelineOptions{Limit: c.QueryInt("limit"), MaxID: c.Query("max_id"), LocalOnly: c.QueryBool("local"), RemoteOnly: c.QueryBool("remote")}
}

func timelineItemsToStatuses(items []mastodonUC.TimelineItem) []statusResponse {
	statuses := make([]statusResponse, 0, len(items))
	for _, item := range items {
		status := noteToStatus(item.Note, &item.Account)
		if item.ID != "" {
			status.ID = item.ID
		}
		if item.URI != "" {
			status.URI = item.URI
			status.URL = item.URI
		}
		if !item.CreatedAt.IsZero() {
			status.CreatedAt = item.CreatedAt.UTC().Format(time.RFC3339)
		}
		status.InReplyToAccountID = item.InReplyToAccountID
		status.MediaAttachments = mediaResponses(item.Media)
		status.Mentions = mentionResponses(item.Mentions)
		status.ReblogsCount = item.ReblogsCount
		status.Reblogged = item.Reblogged
		status.Favourited = item.Favourited
		status.Bookmarked = item.Bookmarked
		status.Pinned = item.Pinned
		if item.Reblog != nil {
			reblog := timelineItemsToStatuses([]mastodonUC.TimelineItem{*item.Reblog})[0]
			status.Content = ""
			status.MediaAttachments = []mediaAttachmentResponse{}
			status.Reblog = &reblog
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func mentionResponses(mentions []models.Mention) []mentionResponse {
	res := make([]mentionResponse, 0, len(mentions))
	for _, mention := range mentions {
		res = append(res, mentionResponse{ID: mention.AccountID, Username: mention.Username, Acct: mention.Acct, URL: mention.URL})
	}
	return res
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
	var editedAt *string
	if note.EditedAt != nil {
		formatted := note.EditedAt.UTC().Format(time.RFC3339)
		editedAt = &formatted
	}
	return statusResponse{ID: note.ID, URI: note.URI, URL: note.URI, CreatedAt: created.UTC().Format(time.RFC3339), EditedAt: editedAt, Account: accountToResponse(account), Content: note.Content, Visibility: visibility, InReplyToID: note.InReplyToID, Sensitive: note.Sensitive, SpoilerText: note.SpoilerText, MediaAttachments: []mediaAttachmentResponse{}, Mentions: []mentionResponse{}, Tags: []any{}, Emojis: []any{}}
}
