package clientapi

import (
	"github.com/gofiber/fiber/v2"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

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
	items, derr := h.conversationsWorkflow.Conversations(c.UserContext(), principal.Account, c.QueryInt("limit"), c.Query("max_id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	resp := make([]conversationResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, conversationResponse{ID: item.ID, Unread: item.Unread, Accounts: accountsToResponses(item.Accounts), LastStatus: timelineItemsToStatuses([]clientapiUC.TimelineItem{item.LastStatus})[0]})
	}
	return c.JSON(resp)
}

func (h APIHandler) deleteConversation(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.conversationsWorkflow.DismissConversation(c.UserContext(), principal.Account, c.Params("id")); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(map[string]any{})
}

func (h APIHandler) readConversation(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.conversationsWorkflow.ReadConversation(c.UserContext(), principal.Account, c.Params("id")); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(map[string]any{})
}
