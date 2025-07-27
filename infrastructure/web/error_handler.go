package web

import (
	"github.com/gofiber/fiber/v2"
	errors "github.com/myfedi/gargoyle/domain/models/domainerrors"
)

func HandleDomainError(c *fiber.Ctx, err *errors.DomainError) error {
	switch err.Code {
	case errors.ErrBadRequest:
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	case errors.ErrInternal:
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	case errors.ErrNotFound:
		return c.Status(fiber.StatusNotFound).SendString(err.Error())
	case errors.ErrUnauthorized:
		return c.Status(fiber.StatusUnauthorized).SendString(err.Error())
	}

	return c.Status(fiber.StatusInternalServerError).SendString("unknown error code or no error given")
}
