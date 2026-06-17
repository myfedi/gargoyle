package clientapi

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/infrastructure/db"
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

type adminRelayRequest struct {
	ActorURI string `json:"actor_uri"`
}

type relayResponse struct {
	ID         string  `json:"id"`
	ActorURI   string  `json:"actor_uri"`
	InboxURI   string  `json:"inbox_uri"`
	Status     string  `json:"status"`
	AcceptedAt *string `json:"accepted_at"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
	LastError  *string `json:"last_error"`
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

func (h APIHandler) adminRelays(c *fiber.Ctx) error {
	principal, derr := h.authenticateAdmin(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	relays, derr := h.moderationWorkflow.ListRelays(c.UserContext(), principal.User)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	resp := make([]relayResponse, 0, len(relays))
	for _, relay := range relays {
		resp = append(resp, relayToResponse(relay))
	}
	return c.JSON(resp)
}

func (h APIHandler) adminCreateRelay(c *fiber.Ctx) error {
	principal, derr := h.authenticateAdmin(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req adminRelayRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	followID, err := db.NewULID()
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	res, derr := h.moderationWorkflow.AddRelay(c.UserContext(), principal.User, principal.Account, clientapiUC.AddRelayInput{ActorURI: req.ActorURI, FollowID: followID})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if res.Inbox != "" {
		if err := h.queueDelivery(res.RawJSON, res.Inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(relayToResponse(res.Relay))
}

func (h APIHandler) adminDisableRelay(c *fiber.Ctx) error {
	principal, derr := h.authenticateAdmin(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	undoID, err := db.NewULID()
	if err != nil {
		return web.HandleDomainError(c, domainerrors.NewErr(domainerrors.ErrInternal, err))
	}
	res, derr := h.moderationWorkflow.DisableRelay(c.UserContext(), principal.User, principal.Account, c.Params("id"), undoID)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if res.Inbox != "" {
		if err := h.queueDelivery(res.RawJSON, res.Inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(relayToResponse(res.Relay))
}

func (h APIHandler) adminDeleteRelay(c *fiber.Ctx) error {
	principal, derr := h.authenticateAdmin(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.moderationWorkflow.DeleteRelay(c.UserContext(), principal.User, c.Params("id")); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func domainBlockToResponse(block models.DomainBlock) domainBlockResponse {
	return domainBlockResponse{ID: block.ID, Domain: block.Domain, Severity: block.Severity, RejectMedia: block.RejectMedia, PublicComment: block.PublicComment, PrivateComment: block.PrivateComment, CreatedByUserID: block.CreatedByUserID, CreatedAt: block.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: block.UpdatedAt.UTC().Format(time.RFC3339)}
}

func relayToResponse(relay models.RelaySubscription) relayResponse {
	var acceptedAt *string
	if relay.AcceptedAt != nil {
		value := relay.AcceptedAt.UTC().Format(time.RFC3339)
		acceptedAt = &value
	}
	return relayResponse{ID: relay.ID, ActorURI: relay.ActorURI, InboxURI: relay.InboxURI, Status: relay.Status, AcceptedAt: acceptedAt, CreatedAt: relay.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: relay.UpdatedAt.UTC().Format(time.RFC3339), LastError: relay.LastError}
}
