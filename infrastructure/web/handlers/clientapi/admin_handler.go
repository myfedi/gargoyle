package clientapi

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

type adminDomainBlockRequest struct {
	Domain         string `json:"domain"`
	PublicComment  string `json:"public_comment"`
	PrivateComment string `json:"private_comment"`
	RejectMedia    bool   `json:"reject_media"`
}

type domainBlockResponse struct {
	ID              string  `json:"id"`
	Domain          string  `json:"domain"`
	Severity        string  `json:"severity"`
	RejectMedia     bool    `json:"reject_media"`
	PublicComment   *string `json:"public_comment"`
	PrivateComment  *string `json:"private_comment"`
	CreatedByUserID string  `json:"created_by_user_id"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type moderationJobResponse struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
}

func (h APIHandler) adminDomainBlocks(c *fiber.Ctx) error {
	principal, derr := h.authenticateAdmin(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	_ = principal
	blocks, derr := h.moderationWorkflow.ListDomainBlocks(c.UserContext())
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	resp := make([]domainBlockResponse, 0, len(blocks))
	for _, block := range blocks {
		resp = append(resp, domainBlockToResponse(block))
	}
	return c.JSON(resp)
}

func (h APIHandler) adminCreateDomainBlock(c *fiber.Ctx) error {
	principal, derr := h.authenticateAdmin(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req adminDomainBlockRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	block, derr := h.moderationWorkflow.CreateDomainBlock(c.UserContext(), principal.User, clientapiUC.CreateDomainBlockInput{Domain: req.Domain, PublicComment: req.PublicComment, PrivateComment: req.PrivateComment, RejectMedia: req.RejectMedia})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(domainBlockToResponse(*block))
}

func (h APIHandler) adminDeleteDomainBlock(c *fiber.Ctx) error {
	principal, derr := h.authenticateAdmin(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.moderationWorkflow.DeleteDomainBlock(c.UserContext(), principal.User, c.Params("domain")); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h APIHandler) adminPurgeDomain(c *fiber.Ctx) error {
	principal, derr := h.authenticateAdmin(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	res, derr := h.moderationWorkflow.EnqueuePurgeDomain(c.UserContext(), principal.User, c.Params("domain"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(moderationJobResponse{ID: res.Job.ID, Kind: res.Job.Kind, Status: string(res.Job.Status)})
}

func domainBlockToResponse(block models.DomainBlock) domainBlockResponse {
	return domainBlockResponse{ID: block.ID, Domain: block.Domain, Severity: block.Severity, RejectMedia: block.RejectMedia, PublicComment: block.PublicComment, PrivateComment: block.PrivateComment, CreatedByUserID: block.CreatedByUserID, CreatedAt: block.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: block.UpdatedAt.UTC().Format(time.RFC3339)}
}
