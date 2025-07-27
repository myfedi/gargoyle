package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/usecases/hostmeta"
)

type HostMetaWebHandlerConfig struct {
	Host string
}

type HostMetaWebHandler struct {
	cfg     HostMetaWebHandlerConfig
	handler hostmeta.HostMetaHandler
}

// NewWebfingerWebHandler creates a new Webfinger handler with the given dependencies.
func NewHostMetaWebHandler(cfg HostMetaWebHandlerConfig) *HostMetaWebHandler {
	handler := hostmeta.NewHostMetaHandler(cfg.Host)
	return &HostMetaWebHandler{
		cfg:     cfg,
		handler: *handler,
	}
}

// SetupHostMeta initializes the hostmeta route for the Fiber application.
func (h *HostMetaWebHandler) SetupHostMeta(app *fiber.App) {
	app.Get("/.well-known/host-meta", func(c *fiber.Ctx) error {
		hostMeta, err := h.handler.HandleHostMetaXML()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		return c.SendString(hostMeta)
	})
}
