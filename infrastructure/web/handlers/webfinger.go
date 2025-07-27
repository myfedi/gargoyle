package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	"github.com/myfedi/gargoyle/domain/usecases/webfinger"
	"github.com/myfedi/gargoyle/infrastructure/web"
)

type WebfingerWebHandlerConfig struct {
	Domain    string
	Host      string
	UsersRepo repos.UsersRepository
}

type WebfingerWebHandler struct {
	cfg     WebfingerWebHandlerConfig
	handler *webfinger.WebfingerHandler
}

// NewWebfingerWebHandler creates a new Webfinger handler with the given dependencies.
func NewWebfingerWebHandler(cfg WebfingerWebHandlerConfig) *WebfingerWebHandler {
	handler := webfinger.NewWebfingerHandler(webfinger.WebFingerHandlerConfig{
		Domain:    cfg.Domain,
		Host:      cfg.Host,
		UsersRepo: cfg.UsersRepo,
	})
	return &WebfingerWebHandler{
		cfg:     cfg,
		handler: handler,
	}
}

// SetupWebfinger initializes the webfinger routes for the Fiber application.
func (h *WebfingerWebHandler) SetupWebfinger(app *fiber.App) {
	app.Get("/.well-known/webfinger", func(c *fiber.Ctx) error {
		c.Accepts("application/jrd+json")

		qry := c.Query("resource")
		if qry == "" {
			return c.Status(fiber.StatusBadRequest).SendString("Missing resource query parameter")
		}

		res, err := h.handler.HandleWebfinger(qry)
		if err != nil {
			return web.HandleDomainError(c, err)
		}

		c.Set("content-type", "application/jrd+json")
		return c.SendString(res)
	})
}
