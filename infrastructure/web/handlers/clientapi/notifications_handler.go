package clientapi

import (
	"time"

	"github.com/gofiber/fiber/v2"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
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
	resp := make([]notificationResponse, 0, len(items))
	for _, item := range items {
		var status *statusResponse
		if item.Status != nil {
			s := timelineItemsToStatuses([]clientapiUC.TimelineItem{*item.Status})[0]
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
