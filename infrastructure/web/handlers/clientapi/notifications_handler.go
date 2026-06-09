package clientapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

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
	items, derr := h.notificationsWorkflow.Notifications(c.UserContext(), principal.Account, c.QueryInt("limit"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(notificationItemsToResponses(items))
}

func (h APIHandler) clearNotifications(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.notificationsWorkflow.ClearNotifications(c.UserContext(), principal.Account); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(map[string]any{})
}

func (h APIHandler) dismissNotification(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.notificationsWorkflow.DismissNotification(c.UserContext(), principal.Account, c.Params("id")); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(map[string]any{})
}
