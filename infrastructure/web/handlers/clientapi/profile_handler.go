package clientapi

import (
	"io"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

type updateCredentialsRequest struct {
	DisplayName string                       `json:"display_name" form:"display_name"`
	Note        string                       `json:"note" form:"note"`
	Fields      []models.AccountProfileField `json:"fields"`
	Locked      bool                         `json:"locked" form:"locked"`
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
	fields := req.Fields
	if formFields := profileFieldsFromForm(c); len(formFields) > 0 {
		fields = formFields
	}
	input := clientapiUC.UpdateCredentialsInput{DisplayName: req.DisplayName, Note: req.Note, Fields: fields, Locked: req.Locked}
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

func profileFieldsFromForm(c *fiber.Ctx) []models.AccountProfileField {
	fields := make([]models.AccountProfileField, 0, 4)
	for i := 0; i < 16; i++ {
		index := strconv.Itoa(i)
		name := firstFormValue(c, "fields_attributes["+index+"][name]", "fields["+index+"][name]")
		value := firstFormValue(c, "fields_attributes["+index+"][value]", "fields["+index+"][value]")
		if name == "" && value == "" {
			continue
		}
		fields = append(fields, models.AccountProfileField{Name: name, Value: value})
	}
	return fields
}

func firstFormValue(c *fiber.Ctx, keys ...string) string {
	for _, key := range keys {
		if value := c.FormValue(key); value != "" {
			return value
		}
	}
	return ""
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
