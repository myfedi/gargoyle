package clientapi

import (
	"io"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

func (h APIHandler) uploadMedia(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	file, err := c.FormFile("file")
	if err != nil {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrBadRequest, "media file is required"))
	}
	opened, err := file.Open()
	if err != nil {
		return err
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, clientapiUC.MaxMediaUploadBytes+1))
	if err != nil {
		return err
	}
	if len(data) > clientapiUC.MaxMediaUploadBytes {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrBadRequest, "media file is too large"))
	}
	media, derr := h.mediaWorkflow.UploadMedia(c.UserContext(), principal.Account, clientapiUC.UploadMediaInput{FileName: file.Filename, ContentType: file.Header.Get("Content-Type"), Data: data, Description: c.FormValue("description")})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(h.mediaResponse(media))
}

type updateMediaRequest struct {
	Description string `json:"description" form:"description"`
}

func (h APIHandler) getMediaAttachment(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	media, derr := h.mediaWorkflow.GetMedia(c.UserContext(), c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if media.LocalAccountID != principal.Account.ID {
		return web.HandleDomainError(c, domainerrors.New(domainerrors.ErrNotFound, mediaNotFoundMessage))
	}
	return c.JSON(h.mediaResponse(media))
}

func (h APIHandler) updateMedia(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req updateMediaRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	media, derr := h.mediaWorkflow.UpdateMedia(c.UserContext(), principal.Account, c.Params("id"), clientapiUC.UpdateMediaInput{Description: req.Description})
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(h.mediaResponse(media))
}

func (h APIHandler) deleteMedia(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	if derr := h.mediaWorkflow.DeleteMedia(c.UserContext(), principal.Account, c.Params("id")); derr != nil {
		return web.HandleDomainError(c, derr)
	}
	return c.JSON(map[string]any{})
}

func (h APIHandler) media(c *fiber.Ctx) error {
	media, derr := h.mediaWorkflow.GetMedia(c.UserContext(), c.Params("id"))
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	c.Set(fiber.HeaderContentType, media.ContentType)
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	return c.Send(media.Data)
}

func (h APIHandler) mediaResponse(media *models.MediaAttachment) mediaAttachmentResponse {
	return mediaResponse(*media)
}

func mediaResponses(media []models.MediaAttachment) []mediaAttachmentResponse {
	res := make([]mediaAttachmentResponse, 0, len(media))
	for _, item := range media {
		res = append(res, mediaResponse(item))
	}
	return res
}

func mediaResponse(media models.MediaAttachment) mediaAttachmentResponse {
	url := "/media/" + media.ID
	return mediaAttachmentResponse{ID: media.ID, Type: mediaType(media.ContentType), URL: url, PreviewURL: url, Description: media.Description}
}

func mediaType(contentType string) string {
	if strings.HasPrefix(contentType, "video/") {
		return "video"
	}
	if strings.HasPrefix(contentType, "audio/") {
		return "audio"
	}
	return "image"
}

func uniqueInboxes(inboxes []string) []string {
	res := make([]string, 0, len(inboxes))
	seen := map[string]bool{}
	for _, inbox := range inboxes {
		if inbox == "" || seen[inbox] {
			continue
		}
		seen[inbox] = true
		res = append(res, inbox)
	}
	return res
}
