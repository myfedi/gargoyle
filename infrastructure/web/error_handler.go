package web

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	errors "github.com/myfedi/gargoyle/domain/models/domainerrors"
)

type errorResponse struct {
	Error string `json:"error"`
}

func HandleDomainError(c *fiber.Ctx, err *errors.DomainError) error {
	status, message := domainHTTPStatusAndMessage(err)
	if wantsAPIError(c) {
		return c.Status(status).JSON(errorResponse{Error: message})
	}
	return c.Status(status).SendString(message)
}

func domainHTTPStatusAndMessage(err *errors.DomainError) (int, string) {
	if err == nil {
		return fiber.StatusInternalServerError, "unknown error"
	}

	switch err.Code {
	case errors.ErrBadRequest:
		return fiber.StatusBadRequest, publicErrorMessage(err)
	case errors.ErrInternal:
		return fiber.StatusInternalServerError, errors.ErrInternal.Error()
	case errors.ErrNotFound:
		return fiber.StatusNotFound, publicErrorMessage(err)
	case errors.ErrUnauthorized:
		return fiber.StatusUnauthorized, publicErrorMessage(err)
	default:
		return fiber.StatusInternalServerError, "unknown error code"
	}
}

func publicErrorMessage(err *errors.DomainError) string {
	if err.Message != "" {
		return err.Message
	}
	if err.Code != nil {
		return err.Code.Error()
	}
	return "unknown error"
}

func wantsAPIError(c *fiber.Ctx) bool {
	path := c.Path()
	if strings.HasPrefix(path, "/api/") || path == "/oauth/token" || path == "/oauth/revoke" {
		return true
	}
	accept := c.Get(fiber.HeaderAccept)
	return strings.Contains(accept, fiber.MIMEApplicationJSON)
}
