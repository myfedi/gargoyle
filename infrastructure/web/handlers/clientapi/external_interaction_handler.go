package clientapi

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

func (h APIHandler) externalInteractionPage(c *fiber.Ctx) error {
	params := url.Values{}
	if uri := c.Query("uri"); uri != "" {
		params.Set("uri", uri)
	}
	target := "/#/external-interaction"
	if encoded := params.Encode(); encoded != "" {
		target += "?" + encoded
	}
	return c.Redirect(target, fiber.StatusFound)
}

type externalInteractionResponse struct {
	Type    string           `json:"type"`
	Account *accountResponse `json:"account,omitempty"`
}

func (h APIHandler) externalInteractionResolve(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	res, derr := h.externalInteraction.Resolve(c.UserContext(), principal.Account, c.Query("uri"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	response := externalInteractionResponse{Type: res.Type}
	if res.Type == clientapiUC.ExternalInteractionTypeAccount && res.Account != nil {
		account := accountToResponse(res.Account)
		response.Account = &account
	}
	return c.JSON(response)
}
