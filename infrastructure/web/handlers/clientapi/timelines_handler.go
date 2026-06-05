package clientapi

import (
	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

func (h APIHandler) homeTimeline(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	items, derr := h.timelinesWorkflow.HomeTimeline(c.UserContext(), principal.Account, timelineOptions(c))
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
	items, derr := h.timelinesWorkflow.PublicTimeline(c.UserContext(), principal.Account, timelineOptions(c))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(timelineItemsToStatuses(items))
}
