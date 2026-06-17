package clientapi

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

type mediaAttachmentResponse struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	PreviewURL  string `json:"preview_url"`
	Description string `json:"description"`
}

type pollRequest struct {
	Options   []string `json:"options" form:"options"`
	ExpiresIn int      `json:"expires_in" form:"expires_in"`
	Multiple  bool     `json:"multiple" form:"multiple"`
}

type createStatusRequest struct {
	Status      string      `json:"status" form:"status"`
	Visibility  string      `json:"visibility" form:"visibility"`
	InReplyToID string      `json:"in_reply_to_id" form:"in_reply_to_id"`
	Sensitive   bool        `json:"sensitive" form:"sensitive"`
	SpoilerText string      `json:"spoiler_text" form:"spoiler_text"`
	MediaIDs    []string    `json:"media_ids" form:"media_ids"`
	ObjectType  string      `json:"activitypub_type" form:"activitypub_type"`
	Poll        pollRequest `json:"poll" form:"poll"`
}

type updateStatusRequest struct {
	Status      string      `json:"status" form:"status"`
	Visibility  string      `json:"visibility" form:"visibility"`
	Sensitive   bool        `json:"sensitive" form:"sensitive"`
	SpoilerText string      `json:"spoiler_text" form:"spoiler_text"`
	MediaIDs    []string    `json:"media_ids" form:"media_ids"`
	ObjectType  string      `json:"activitypub_type" form:"activitypub_type"`
	Poll        pollRequest `json:"poll" form:"poll"`
}

func mergeFormPoll(c *fiber.Ctx, poll *pollRequest) {
	args := c.Request().PostArgs()
	args.VisitAll(func(key, value []byte) {
		switch string(key) {
		case "poll[options][]", "poll[options]":
			poll.Options = append(poll.Options, string(value))
		case "poll[expires_in]":
			if parsed, err := strconv.Atoi(string(value)); err == nil {
				poll.ExpiresIn = parsed
			}
		case "poll[multiple]":
			poll.Multiple = truthyFormValue(string(value))
		}
	})
}

func truthyFormValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (h APIHandler) createStatus(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req createStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	mergeFormPoll(c, &req.Poll)
	res, derr := h.statusesWorkflow.CreateStatus(c.UserContext(), principal.Account, clientapiUC.CreateStatusInput{Content: req.Status, Visibility: req.Visibility, InReplyToID: req.InReplyToID, Sensitive: req.Sensitive, SpoilerText: req.SpoilerText, MediaIDs: req.MediaIDs, ObjectType: req.ObjectType, PollOptions: req.Poll.Options, PollMultiple: req.Poll.Multiple, PollExpiresIn: req.Poll.ExpiresIn})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	for _, inbox := range uniqueInboxes(append(append(res.FollowerInboxes, res.MentionInboxes...), res.RelayInboxes...)) {
		if err := h.queueDelivery(res.RawJSON, inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	item, derr := h.statusesWorkflow.GetStatus(c.UserContext(), principal.Account, res.Note.ID)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	status := timelineItemsToStatuses([]clientapiUC.TimelineItem{*item})[0]
	return c.JSON(status)
}

func (h APIHandler) updateStatus(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req updateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	mergeFormPoll(c, &req.Poll)
	res, derr := h.statusesWorkflow.UpdateStatus(c.UserContext(), principal.Account, c.Params("id"), clientapiUC.UpdateStatusInput{Content: req.Status, Visibility: req.Visibility, Sensitive: req.Sensitive, SpoilerText: req.SpoilerText, MediaIDs: req.MediaIDs, ObjectType: req.ObjectType, PollOptions: req.Poll.Options, PollMultiple: req.Poll.Multiple, PollExpiresIn: req.Poll.ExpiresIn})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	for _, inbox := range uniqueInboxes(append(append(res.FollowerInboxes, res.MentionInboxes...), res.RelayInboxes...)) {
		if err := h.queueDelivery(res.RawJSON, inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	item, derr := h.statusesWorkflow.GetStatus(c.UserContext(), principal.Account, res.Note.ID)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	status := timelineItemsToStatuses([]clientapiUC.TimelineItem{*item})[0]
	return c.JSON(status)
}

func (h APIHandler) status(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	item, derr := h.statusesWorkflow.GetStatus(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	statuses := timelineItemsToStatuses([]clientapiUC.TimelineItem{*item})
	return c.JSON(statuses[0])
}

type statusSourceResponse struct {
	ID              string `json:"id"`
	Text            string `json:"text"`
	SpoilerText     string `json:"spoiler_text"`
	ActivityPubType string `json:"activitypub_type"`
}

type statusHistoryResponse struct {
	Content          string                    `json:"content"`
	SpoilerText      string                    `json:"spoiler_text"`
	Sensitive        bool                      `json:"sensitive"`
	ActivityPubType  string                    `json:"activitypub_type"`
	CreatedAt        string                    `json:"created_at"`
	Account          accountResponse           `json:"account"`
	MediaAttachments []mediaAttachmentResponse `json:"media_attachments"`
	Emojis           []any                     `json:"emojis"`
}

func (h APIHandler) statusSource(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	item, derr := h.statusesWorkflow.GetStatus(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(statusSourceResponse{ID: item.Note.ID, Text: item.Note.PlainText, SpoilerText: item.Note.SpoilerText, ActivityPubType: normalizedResponseObjectType(item.Note.ObjectType)})
}

func (h APIHandler) statusHistory(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	history, derr := h.statusesWorkflow.StatusHistory(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	resp := make([]statusHistoryResponse, 0, len(history))
	for _, item := range history {
		resp = append(resp, statusHistoryResponse{Content: item.Content, SpoilerText: item.SpoilerText, Sensitive: item.Sensitive, ActivityPubType: normalizedResponseObjectType(item.ObjectType), CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339), Account: accountToResponse(&item.Account), MediaAttachments: mediaResponses(item.Media), Emojis: []any{}})
	}
	return c.JSON(resp)
}

func (h APIHandler) deleteStatus(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	res, derr := h.statusesWorkflow.DeleteStatus(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	for _, inbox := range res.FollowerInboxes {
		if err := h.queueDelivery(res.RawJSON, inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.SendStatus(fiber.StatusOK)
}

type contextResponse struct {
	Ancestors   []statusResponse `json:"ancestors"`
	Descendants []statusResponse `json:"descendants"`
	Warnings    []string         `json:"warnings,omitempty"`
}

func (h APIHandler) statusContext(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	ctxResp, derr := h.statusesWorkflow.StatusContext(c.UserContext(), principal.Account, c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(contextResponse{Ancestors: timelineItemsToStatuses(ctxResp.Ancestors), Descendants: timelineItemsToStatuses(ctxResp.Descendants), Warnings: ctxResp.Warnings})
}
