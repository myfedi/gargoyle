package clientapi

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/domain/usecases/oauth"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

// APIHandler exposes the small client API surface needed by web
// and mobile clients after OAuth login.
type DeliveryQueue func(body []byte, inbox string, account models.Account) *domainerrors.DomainError

const pushSubscriptionRoute = "/api/v1/push/subscription"

type APIHandler struct {
	oauth                 oauth.UseCase
	instanceWorkflow      clientapiUC.Instance
	accountsWorkflow      clientapiUC.Accounts
	statusesWorkflow      clientapiUC.Statuses
	timelinesWorkflow     clientapiUC.Timelines
	interactionsWorkflow  clientapiUC.Interactions
	externalInteraction   clientapiUC.ExternalInteraction
	notificationsWorkflow clientapiUC.Notifications
	conversationsWorkflow clientapiUC.Conversations
	mediaWorkflow         clientapiUC.Media
	profileWorkflow       clientapiUC.Profile
	moderationWorkflow    clientapiUC.Moderation
	pushRepo              repos.PushSubscriptionRepository
	vapidPublicKey        string
	queueDelivery         DeliveryQueue
	realtimeHub           *RealtimeHub
}

type APIHandlerConfig struct {
	OAuth               oauth.UseCase
	Instance            clientapiUC.Instance
	Accounts            clientapiUC.Accounts
	Statuses            clientapiUC.Statuses
	Timelines           clientapiUC.Timelines
	Interactions        clientapiUC.Interactions
	ExternalInteraction clientapiUC.ExternalInteraction
	Notifications       clientapiUC.Notifications
	Conversations       clientapiUC.Conversations
	Media               clientapiUC.Media
	Profile             clientapiUC.Profile
	Moderation          clientapiUC.Moderation
	PushRepo            repos.PushSubscriptionRepository
	VAPIDPublicKey      string
	QueueDelivery       DeliveryQueue
	RealtimeHub         *RealtimeHub
}

func NewAPIHandler(cfg APIHandlerConfig) APIHandler {
	if cfg.QueueDelivery == nil {
		panic("client API handler requires QueueDelivery")
	}
	realtimeHub := cfg.RealtimeHub
	if realtimeHub == nil {
		realtimeHub = NewRealtimeHub(cfg.Accounts, cfg.Notifications)
	}
	return APIHandler{oauth: cfg.OAuth, instanceWorkflow: cfg.Instance, accountsWorkflow: cfg.Accounts, statusesWorkflow: cfg.Statuses, timelinesWorkflow: cfg.Timelines, interactionsWorkflow: cfg.Interactions, externalInteraction: cfg.ExternalInteraction, notificationsWorkflow: cfg.Notifications, conversationsWorkflow: cfg.Conversations, mediaWorkflow: cfg.Media, profileWorkflow: cfg.Profile, moderationWorkflow: cfg.Moderation, pushRepo: cfg.PushRepo, vapidPublicKey: cfg.VAPIDPublicKey, queueDelivery: cfg.QueueDelivery, realtimeHub: realtimeHub}
}

func (h APIHandler) Setup(app *fiber.App) {
	app.Get("/share", h.sharePageRedirect)
	app.Get("/api/v1/instance", h.instanceV1)
	app.Get("/api/v2/instance", h.instanceV2)
	app.Get("/api/v2/search", h.search)
	app.Get("/api/v1/external_interaction", h.externalInteractionResolve)
	app.Get("/activitypub/externalInteraction", h.externalInteractionPage)
	app.Get("/api/v1/accounts/search", h.accountsSearch)
	app.Get("/api/v1/accounts/relationships", h.relationships)
	app.Get("/api/v1/admin/domain_blocks", h.adminDomainBlocks)
	app.Post("/api/v1/admin/domain_blocks", h.adminCreateDomainBlock)
	app.Delete("/api/v1/admin/domain_blocks/:domain", h.adminDeleteDomainBlock)
	app.Post("/api/v1/admin/domain_blocks/:domain/purge", h.adminPurgeDomain)
	app.Get("/api/v1/admin/relays", h.adminRelays)
	app.Post("/api/v1/admin/relays", h.adminCreateRelay)
	app.Post("/api/v1/admin/relays/:id/disable", h.adminDisableRelay)
	app.Delete("/api/v1/admin/relays/:id", h.adminDeleteRelay)
	app.Get("/api/v1/streaming", h.streaming)
	app.Get("/api/v1/streaming/user/notification", h.notificationStream)
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
	app.Post(pushSubscriptionRoute, h.createPushSubscription)
	app.Get(pushSubscriptionRoute, h.getPushSubscription)
	app.Put(pushSubscriptionRoute, h.updatePushSubscription)
	app.Delete(pushSubscriptionRoute, h.deletePushSubscription)
	app.Get("/api/v1/announcements", h.emptyList)
	app.Get("/api/v1/trends/tags", h.emptyList)
	app.Get("/api/v1/trends/statuses", h.emptyList)
	app.Get("/api/v1/trends/links", h.emptyList)
	app.Get("/api/v1/lists", h.emptyList)
	app.Get("/api/v1/filters", h.emptyList)
	app.Get("/api/v2/filters", h.emptyList)
	app.Post("/api/v2/media", h.uploadMedia)
	app.Post("/api/v1/media", h.uploadMedia)
	app.Get(mastodonMediaRoute, h.getMediaAttachment)
	app.Put(mastodonMediaRoute, h.updateMedia)
	app.Delete(mastodonMediaRoute, h.deleteMedia)
	app.Get("/media/:id", h.media)
	app.Get("/media/:id/:filename", h.media)
	app.Head("/media/:id", h.media)
	app.Head("/media/:id/:filename", h.media)
	app.Get("/api/v1/follow_requests", h.followRequests)
	app.Post("/api/v1/follow_requests/:id/authorize", h.authorizeFollowRequest)
	app.Post("/api/v1/follow_requests/:id/reject", h.rejectFollowRequest)
	app.Get("/api/v1/accounts/:id/followers", h.followers)
	app.Get("/api/v1/accounts/:id/following", h.following)
	app.Get("/api/v1/accounts/:id/statuses", h.accountStatuses)
	app.Patch("/api/v1/accounts/update_credentials", h.updateCredentials)
	app.Post("/api/v1/accounts/:id/follow", h.followAccount)
	app.Post("/api/v1/accounts/:id/unfollow", h.unfollowAccount)
	app.Get("/api/v1/accounts/:id", h.account)
	app.Post("/api/v1/statuses", h.createStatus)
	app.Put(mastodonStatusRoute, h.updateStatus)
	app.Patch(mastodonStatusRoute, h.updateStatus)
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
	app.Post("/api/v1/polls/:id/votes", h.votePoll)
	app.Get(mastodonStatusRoute, h.status)
	app.Delete(mastodonStatusRoute, h.deleteStatus)
	app.Get("/api/v1/timelines/home", h.homeTimeline)
	app.Get("/api/v1/timelines/public", h.publicTimeline)
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

func (h APIHandler) authenticateAdmin(c *fiber.Ctx) (*oauth.AuthenticatedUser, *domainerrors.DomainError) {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return nil, derr
	}
	if principal.User == nil || !principal.User.Admin {
		return nil, domainerrors.New(domainerrors.ErrUnauthorized, "admin access required")
	}
	return principal, nil
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

func scopeIncludes(scopes, required string) bool {
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
