package clientapi

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

type pushSubscriptionRequest struct {
	Subscription struct {
		Endpoint string `json:"endpoint" form:"endpoint"`
		Keys     struct {
			P256DH string `json:"p256dh" form:"p256dh"`
			Auth   string `json:"auth" form:"auth"`
		} `json:"keys"`
	} `json:"subscription"`
	Data struct {
		Alerts pushAlertsRequest `json:"alerts"`
		Policy string            `json:"policy"`
	} `json:"data"`
}

type pushAlertsRequest struct {
	Mention       *bool `json:"mention"`
	Status        *bool `json:"status"`
	Reblog        *bool `json:"reblog"`
	Follow        *bool `json:"follow"`
	FollowRequest *bool `json:"follow_request"`
	Favourite     *bool `json:"favourite"`
	Poll          *bool `json:"poll"`
	Update        *bool `json:"update"`
	AdminSignUp   *bool `json:"admin.sign_up"`
	AdminReport   *bool `json:"admin.report"`
}

type pushSubscriptionResponse struct {
	ID        string          `json:"id"`
	Endpoint  string          `json:"endpoint"`
	ServerKey string          `json:"server_key"`
	Alerts    map[string]bool `json:"alerts"`
	Policy    string          `json:"policy"`
}

func (h APIHandler) createPushSubscription(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "push")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if h.pushRepo == nil {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrInternal, "push subscriptions are not configured"))
	}
	var req pushSubscriptionRequest
	_ = c.BodyParser(&req)
	applyPushFormValues(c, &req)
	if strings.TrimSpace(req.Subscription.Endpoint) == "" || strings.TrimSpace(req.Subscription.Keys.P256DH) == "" || strings.TrimSpace(req.Subscription.Keys.Auth) == "" {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrBadRequest, "push subscription endpoint and keys are required"))
	}
	if err := validatePushEndpoint(req.Subscription.Endpoint); err != nil {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrBadRequest, err.Error()))
	}
	alerts := defaultPushAlerts()
	mergePushAlerts(&alerts, req.Data.Alerts)
	sub, err := h.pushRepo.UpsertPushSubscription(c.UserContext(), nil, repos.UpsertPushSubscriptionInput{LocalAccountID: principal.Account.ID, AccessTokenID: principal.AccessTokenID, Endpoint: req.Subscription.Endpoint, KeyP256DH: req.Subscription.Keys.P256DH, KeyAuth: req.Subscription.Keys.Auth, Policy: req.Data.Policy, Alerts: alerts})
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	return c.JSON(h.pushResponse(sub))
}

func (h APIHandler) getPushSubscription(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "push")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	sub, err := h.pushRepo.GetPushSubscriptionByToken(c.UserContext(), nil, principal.AccessTokenID)
	if err != nil {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrNotFound, "push subscription not found"))
	}
	return c.JSON(h.pushResponse(sub))
}

func (h APIHandler) updatePushSubscription(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "push")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	current, err := h.pushRepo.GetPushSubscriptionByToken(c.UserContext(), nil, principal.AccessTokenID)
	if err != nil {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrNotFound, "push subscription not found"))
	}
	var req pushSubscriptionRequest
	_ = c.BodyParser(&req)
	applyPushFormValues(c, &req)
	alerts := current.Alerts
	mergePushAlerts(&alerts, req.Data.Alerts)
	policy := req.Data.Policy
	if policy == "" {
		policy = current.Policy
	}
	sub, err := h.pushRepo.UpdatePushSubscription(c.UserContext(), nil, principal.AccessTokenID, policy, alerts)
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	return c.JSON(h.pushResponse(sub))
}

func (h APIHandler) deletePushSubscription(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "push")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if err := h.pushRepo.DeletePushSubscriptionByToken(c.UserContext(), nil, principal.AccessTokenID); err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	return c.SendStatus(fiber.StatusOK)
}

func (h APIHandler) pushResponse(sub *models.PushSubscription) pushSubscriptionResponse {
	return pushSubscriptionResponse{ID: sub.ID, Endpoint: sub.Endpoint, ServerKey: h.vapidPublicKey, Policy: sub.Policy, Alerts: map[string]bool{"mention": sub.Alerts.Mention, "status": sub.Alerts.Status, "reblog": sub.Alerts.Reblog, "follow": sub.Alerts.Follow, "follow_request": sub.Alerts.FollowRequest, "favourite": sub.Alerts.Favourite, "poll": sub.Alerts.Poll, "update": sub.Alerts.Update, "admin.sign_up": sub.Alerts.AdminSignUp, "admin.report": sub.Alerts.AdminReport}}
}

func validatePushEndpoint(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return domainerrors.New(domainerrors.ErrBadRequest, "invalid push endpoint URL")
	}
	if parsed.Scheme != "https" {
		return domainerrors.New(domainerrors.ErrBadRequest, "push endpoint must use https")
	}
	return nil
}

func defaultPushAlerts() models.PushAlerts {
	return models.PushAlerts{Mention: true, Reblog: true, Follow: true, FollowRequest: true, Favourite: true, Poll: true}
}
func mergePushAlerts(alerts *models.PushAlerts, req pushAlertsRequest) {
	if req.Mention != nil {
		alerts.Mention = *req.Mention
	}
	if req.Status != nil {
		alerts.Status = *req.Status
	}
	if req.Reblog != nil {
		alerts.Reblog = *req.Reblog
	}
	if req.Follow != nil {
		alerts.Follow = *req.Follow
	}
	if req.FollowRequest != nil {
		alerts.FollowRequest = *req.FollowRequest
	}
	if req.Favourite != nil {
		alerts.Favourite = *req.Favourite
	}
	if req.Poll != nil {
		alerts.Poll = *req.Poll
	}
	if req.Update != nil {
		alerts.Update = *req.Update
	}
	if req.AdminSignUp != nil {
		alerts.AdminSignUp = *req.AdminSignUp
	}
	if req.AdminReport != nil {
		alerts.AdminReport = *req.AdminReport
	}
}
func boolPtrFromForm(c *fiber.Ctx, name string) *bool {
	v := c.FormValue(name)
	if v == "" {
		return nil
	}
	b := v == "true" || v == "1" || v == "on"
	return &b
}
func applyPushFormValues(c *fiber.Ctx, req *pushSubscriptionRequest) {
	if v := c.FormValue("subscription[endpoint]"); v != "" {
		req.Subscription.Endpoint = v
	}
	if v := c.FormValue("subscription[keys][p256dh]"); v != "" {
		req.Subscription.Keys.P256DH = v
	}
	if v := c.FormValue("subscription[keys][auth]"); v != "" {
		req.Subscription.Keys.Auth = v
	}
	if v := c.FormValue("data[policy]"); v != "" {
		req.Data.Policy = v
	}
	if b := boolPtrFromForm(c, "data[alerts][mention]"); b != nil {
		req.Data.Alerts.Mention = b
	}
	if b := boolPtrFromForm(c, "data[alerts][status]"); b != nil {
		req.Data.Alerts.Status = b
	}
	if b := boolPtrFromForm(c, "data[alerts][reblog]"); b != nil {
		req.Data.Alerts.Reblog = b
	}
	if b := boolPtrFromForm(c, "data[alerts][follow]"); b != nil {
		req.Data.Alerts.Follow = b
	}
	if b := boolPtrFromForm(c, "data[alerts][follow_request]"); b != nil {
		req.Data.Alerts.FollowRequest = b
	}
	if b := boolPtrFromForm(c, "data[alerts][favourite]"); b != nil {
		req.Data.Alerts.Favourite = b
	}
	if b := boolPtrFromForm(c, "data[alerts][poll]"); b != nil {
		req.Data.Alerts.Poll = b
	}
	if b := boolPtrFromForm(c, "data[alerts][update]"); b != nil {
		req.Data.Alerts.Update = b
	}
}
