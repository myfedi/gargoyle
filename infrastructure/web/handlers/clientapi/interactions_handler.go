package clientapi

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

func (h APIHandler) favouriteStatuses(c *fiber.Ctx) error {
	return h.interactionTimeline(c, h.interactionsWorkflow.FavouriteStatuses)
}

func (h APIHandler) bookmarkedStatuses(c *fiber.Ctx) error {
	return h.interactionTimeline(c, h.interactionsWorkflow.BookmarkedStatuses)
}

func (h APIHandler) interactionTimeline(c *fiber.Ctx, fn func(context.Context, *models.Account, int) ([]clientapiUC.TimelineItem, *domainerrors.DomainError)) error {
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

type votePollRequest struct {
	Choices []int `json:"choices" form:"choices"`
}

func (h APIHandler) votePoll(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req votePollRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	res, derr := h.interactionsWorkflow.VotePoll(c.UserContext(), principal.Account, c.Params("id"), req.Choices)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	for _, delivery := range res.Deliveries {
		if err := h.queueDelivery(delivery.RawJSON, delivery.Inbox, delivery.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(pollToResponse(res.Poll))
}

func (h APIHandler) favouriteStatus(c *fiber.Ctx) error {
	return h.interactStatus(c, h.interactionsWorkflow.FavouriteStatus)
}
func (h APIHandler) unfavouriteStatus(c *fiber.Ctx) error {
	return h.interactStatus(c, h.interactionsWorkflow.UnfavouriteStatus)
}
func (h APIHandler) bookmarkStatus(c *fiber.Ctx) error {
	return h.localStatusInteraction(c, h.interactionsWorkflow.BookmarkStatus)
}
func (h APIHandler) unbookmarkStatus(c *fiber.Ctx) error {
	return h.localStatusInteraction(c, h.interactionsWorkflow.UnbookmarkStatus)
}
func (h APIHandler) pinStatus(c *fiber.Ctx) error {
	return h.localStatusInteraction(c, h.interactionsWorkflow.PinStatus)
}
func (h APIHandler) unpinStatus(c *fiber.Ctx) error {
	return h.localStatusInteraction(c, h.interactionsWorkflow.UnpinStatus)
}
func (h APIHandler) reblogStatus(c *fiber.Ctx) error {
	return h.interactStatus(c, h.interactionsWorkflow.ReblogStatus)
}
func (h APIHandler) unreblogStatus(c *fiber.Ctx) error {
	return h.interactStatus(c, h.interactionsWorkflow.UnreblogStatus)
}

func (h APIHandler) localStatusInteraction(c *fiber.Ctx, fn func(context.Context, *models.Account, string) (*clientapiUC.TimelineItem, *domainerrors.DomainError)) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	item, derr := fn(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(timelineItemsToStatuses([]clientapiUC.TimelineItem{*item})[0])
}

func (h APIHandler) interactStatus(c *fiber.Ctx, fn func(context.Context, *models.Account, string) (*clientapiUC.InteractionResult, *domainerrors.DomainError)) error {
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
	return c.JSON(timelineItemsToStatuses([]clientapiUC.TimelineItem{res.Status})[0])
}
