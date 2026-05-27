package nodeinfo

import (
	"context"
	"encoding/json"

	"github.com/myfedi/gargoyle/domain/ports/repos"
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

type nodeInfoLinksResponse struct {
	Links []nodeInfoLink `json:"links"`
}

type nodeInfoLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

// HandleNodeInfo processes the nodeinfo request for a given domain.
// This only returns the supported versions and protocols.
// For the actual nodeinfo retrieval, use HandleNodeInfoRetrieval.
func (h *NodeInfoHandler) HandleNodeInfo() (string, error) {
	res, err := json.Marshal(nodeInfoLinksResponse{Links: []nodeInfoLink{
		{Rel: "http://nodeinfo.diaspora.software/ns/schema/2.0", Href: h.cfg.Domain + "/nodeinfo/2.0"},
		{Rel: "http://nodeinfo.diaspora.software/ns/schema/2.1", Href: h.cfg.Domain + "/nodeinfo/2.1"},
	}})
	if err != nil {
		return "", err
	}
	return string(res), nil
}

type nodeInfoResponse struct {
	Version   string           `json:"version"`
	Protocols []string         `json:"protocols"`
	Software  nodeInfoSoftware `json:"software"`
	Usage     nodeInfoUsage    `json:"usage"`
}

type nodeInfoSoftware struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Homepage   string `json:"homepage"`
	Repository string `json:"repository"`
}

type nodeInfoUsage struct {
	Users         int `json:"users"`
	LocalPosts    int `json:"localPosts"`
	LocalComments int `json:"localComments"`
}

func (h *NodeInfoHandler) HandleNodeInfoRetrieval(ctx context.Context, nsVersion string) (string, error) {
	usersCount, err := h.cfg.UsersRepo.GetUsersCount(ctx, nil)
	if err != nil {
		return "", err
	}
	postsCount, err := h.cfg.PostsRepo.GetLocalPostsCount(ctx)
	if err != nil {
		return "", err
	}
	commentsCount, err := h.cfg.CommentsRepo.GetLocalCommentsCount(ctx)
	if err != nil {
		return "", err
	}

	res, err := json.Marshal(nodeInfoResponse{
		Version:   nsVersion,
		Protocols: []string{"activitypub"},
		Software: nodeInfoSoftware{
			Name:       "Gargoyle",
			Version:    h.cfg.ServerVersion,
			Homepage:   "https://github.com/myfedi/gargoyle",
			Repository: "https://github.com/myfedi/gargoyle",
		},
		Usage: nodeInfoUsage{
			Users:         usersCount,
			LocalPosts:    postsCount,
			LocalComments: commentsCount,
		},
	})
	if err != nil {
		return "", err
	}
	return string(res), nil
}
