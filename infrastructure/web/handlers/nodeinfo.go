package handlers

import (
	"fmt"

	fiber "github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	"github.com/myfedi/gargoyle/domain/usecases/nodeinfo"
)

// Adapters for the nodeinfo handlers.
type NodeInfoHandlerConfig struct {
	UsersRepo     repos.UsersRepository
	PostsRepo     repos.PostsRepository
	CommentsRepo  repos.CommentsRepository
	Host          string
	ServerVersion string
}

type NodeInfoWebHandler struct {
	cfg NodeInfoHandlerConfig
}

// NewNodeInfoWebHandler creates a new NodeInfoWebHandler with the given dependencies.
func NewNodeInfoWebHandler(cfg NodeInfoHandlerConfig) *NodeInfoWebHandler {
	return &NodeInfoWebHandler{
		cfg: cfg,
	}
}

// SetupNodeInfo initializes the nodeinfo routes for the Fiber application.
// We support:
// - NodeInfo 2.0 and 2.1 retrieval
// - Redirects for .well-known/nodeinfo/:version
func (h NodeInfoWebHandler) SetupNodeInfo(app *fiber.App) {
	handler := nodeinfo.NewNodeInfoHandler(nodeinfo.NodeInfoHandlerConfig{
		Domain:        h.cfg.Host,
		ServerVersion: h.cfg.ServerVersion,
		UsersRepo:     h.cfg.UsersRepo,
		PostsRepo:     h.cfg.PostsRepo,
		CommentsRepo:  h.cfg.CommentsRepo,
	})

	app.Get("/.well-known/nodeinfo/:version", func(c *fiber.Ctx) error {
		// Redirect to /nodeinfo/:version
		version := c.Params("version")
		if version != "2.0" && version != "2.1" {
			return c.Status(fiber.StatusNotFound).SendString("Unsupported version")
		}
		return c.Redirect(fmt.Sprintf("/nodeinfo/%s", version), fiber.StatusMovedPermanently)
	})

	app.Get("/.well-known/nodeinfo", func(c *fiber.Ctx) error {
		c.Accepts("application/json")

		nodeInfo, err := handler.HandleNodeInfo()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}

		c.Set("content-type", "application/json")
		return c.SendString(nodeInfo)
	})

	// Handle nodeinfo retrieval for specific version
	app.Get("/nodeinfo/:version", func(c *fiber.Ctx) error {
		c.Accepts("application/json")

		version := c.Params("version")
		if version == "" {
			return c.Status(fiber.StatusBadRequest).SendString("Missing version parameter")
		}

		if version != "2.0" && version != "2.1" {
			return c.Status(fiber.StatusBadRequest).SendString("Unsupported version, only 2.0 and 2.1 are supported")
		}

		nodeInfo, err := handler.HandleNodeInfoRetrieval(version)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}

		// note that we checked version above already, so we can just sprintf it into the string
		contentType := fmt.Sprintf("application/json; profile=\"http://nodeinfo.diaspora.software/ns/schema/%s#\"", version)
		c.Set("content-type", contentType)
		return c.SendString(nodeInfo)
	})
}
