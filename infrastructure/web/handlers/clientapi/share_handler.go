package clientapi

import "github.com/gofiber/fiber/v2"

func (h APIHandler) sharePageRedirect(c *fiber.Ctx) error {
	target := "/#/share"
	if query := c.Request().URI().QueryString(); len(query) > 0 {
		target += "?" + string(query)
	}
	return c.Redirect(target, fiber.StatusFound)
}
