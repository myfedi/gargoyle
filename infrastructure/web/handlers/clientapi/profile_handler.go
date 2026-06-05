package clientapi

import (
	"io"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

type updateCredentialsRequest struct {
	DisplayName string `json:"display_name" form:"display_name"`
	Note        string `json:"note" form:"note"`
	Locked      bool   `json:"locked" form:"locked"`
}

func (h APIHandler) updateCredentials(c *fiber.Ctx) error {
	principal, derr := h.authenticate(c, "write")
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	var req updateCredentialsRequest
	if err := c.BodyParser(&req); err != nil {
		return err
	}
	input := clientapiUC.UpdateCredentialsInput{DisplayName: req.DisplayName, Note: req.Note, Locked: req.Locked}
	if avatar, derr := profileUploadFromForm(c, "avatar"); derr != nil {
		return web.HandleDomainError(c, derr)
	} else {
		input.Avatar = avatar
	}
	if header, derr := profileUploadFromForm(c, "header"); derr != nil {
		return web.HandleDomainError(c, derr)
	} else {
		input.Header = header
	}
	res, derr := h.profileWorkflow.UpdateCredentials(c.UserContext(), principal.Account, input)
	if derr != nil {
		return web.HandleDomainError(c, derr)
	}
	for _, inbox := range uniqueInboxes(res.FollowerInboxes) {
		if err := h.queueDelivery(res.RawJSON, inbox, res.Account); err != nil {
			return web.HandleDomainError(c, err)
		}
	}
	return c.JSON(accountToResponse(&res.Account))
}

func profileUploadFromForm(c *fiber.Ctx, field string) (*clientapiUC.UploadMediaInput, *domainerrors.DomainError) {
	file, err := c.FormFile(field)
	if err != nil {
		return nil, nil
	}
	opened, err := file.Open()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, clientapiUC.MaxMediaUploadBytes+1))
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if len(data) > clientapiUC.MaxMediaUploadBytes {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "profile image is too large")
	}
	return &clientapiUC.UploadMediaInput{FileName: file.Filename, ContentType: file.Header.Get("Content-Type"), Data: data}, nil
}
