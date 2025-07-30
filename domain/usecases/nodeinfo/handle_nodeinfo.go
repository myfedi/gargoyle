package nodeinfo

import (
	"github.com/myfedi/gargoyle/domain/ports/repos"
	"github.com/myfedi/gargoyle/utils"
)

type NodeInfoHandlerConfig struct {
	// Domain is the Domain where the server is hosted on.
	Domain string
	// ServerVersion is the version of the server software.
	ServerVersion string

	// Repositories for the nodeinfo usage data
	UsersRepo    repos.UsersRepository
	PostsRepo    repos.PostsRepository
	CommentsRepo repos.CommentsRepository
}

type NodeInfoHandler struct {
	cfg NodeInfoHandlerConfig
}

// NewNodeInfoHandler creates a new NodeInfoHandler with the given domain and server version.
// Provides handlers for the nodeinfo protocol.
// See https://nodeinfo.diaspora.software/protocol.html
func NewNodeInfoHandler(cfg NodeInfoHandlerConfig) *NodeInfoHandler {
	return &NodeInfoHandler{cfg: cfg}
}

// HandleNodeInfo processes the nodeinfo request for a given domain.
// This only returns the supported versions and protocols.
// For the actual nodeinfo retrieval, use HandleNodeInfoRetrieval.
func (h *NodeInfoHandler) HandleNodeInfo() (string, error) {
	return utils.NamedFormat(`{
	"links": [
		{
			"rel": "http://nodeinfo.diaspora.software/ns/schema/2.0",
			"href": "{{.domain}}/nodeinfo/2.0",
		},
		{
			"rel": "http://nodeinfo.diaspora.software/ns/schema/2.1",
			"href": "{{.domain}}/nodeinfo/2.1",
		},
	]
}`, utils.FormatParams{
		"domain": h.cfg.Domain,
	})
}

func (h *NodeInfoHandler) HandleNodeInfoRetrieval(nsVersion string) (string, error) {
	// Retrieve usage data from repositories
	usersCount, err := h.cfg.UsersRepo.GetUsersCount(nil)
	if err != nil {
		return "", err
	}
	postsCount, err := h.cfg.PostsRepo.GetLocalPostsCount()
	if err != nil {
		return "", err
	}
	commentsCount, err := h.cfg.CommentsRepo.GetLocalCommentsCount()
	if err != nil {
		return "", err
	}

	return utils.NamedFormat(`{
	"version": {{.nsVersion}},
	"links": [
		{
			"rel": "http://nodeinfo.diaspora.software/ns/schema/{{.nsVersion}}",
			"href": "{{.domain}}/nodeinfo/{{.nsVersion}}",
			"protocols": ["activitypub"],
			"software": {
				"name": "Gargoyle",
				"version": {{.serverVersion}},
				"homepage": "https://github.com/myfedi/gargoyle",
				"repository": "https://github.com/myfedi/gargoyle"
			},
			"usage": {
				"users": {{.usersCount}},
				"localPosts": {{.postsCount}},
				"localComments": {{.commentsCount}},
			}
		}
	]
}`, utils.FormatParams{
		"nsVersion":     nsVersion,
		"domain":        h.cfg.Domain,
		"serverVersion": h.cfg.ServerVersion,
		"usersCount":    usersCount,
		"postsCount":    postsCount,
		"commentsCount": commentsCount,
	})
}
