package webfinger

import (
	"context"
	"fmt"
	"strings"

	errors "github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type WebFingerHandlerConfig struct {
	Domain string
	Host   string

	UsersRepo repos.UsersRepository
}

type WebfingerHandler struct {
	cfg WebFingerHandlerConfig
}

func NewWebfingerHandler(cfg WebFingerHandlerConfig) *WebfingerHandler {
	if cfg.UsersRepo == nil {
		panic("webfinger handler requires UsersRepo")
	}
	return &WebfingerHandler{
		cfg: cfg,
	}
}

// HandleWebfinger processes the webfinger request for a given resource.
// It expects the resource to be in the format "acct:username@domain". The domain needs to
// match as well.
// Both acct:username@domain and acct://username@domain are supported.
// See https://datatracker.ietf.org/doc/html/rfc7033
func (h *WebfingerHandler) HandleWebfinger(ctx context.Context, resource string) (string, *errors.DomainError) {
	// first we try to parse the resource string.
	// we expect something like "acct:alice@example.org"
	if resource == "" {
		return "", errors.New(errors.ErrBadRequest, "resource cannot be empty")
	}

	split := ":"
	// this is not expected in activitypub, but we still support it anyhow
	if strings.Contains(resource, "://") {
		split = "://"
	}

	parts := strings.SplitN(resource, split, 2)
	if len(parts) != 2 || parts[0] != "acct" {
		return "", errors.New(errors.ErrBadRequest, invalidResourceFormatMessage)
	}

	if parts[0] != "acct" {
		return "", errors.New(errors.ErrBadRequest, "unsupported resource type, expected 'acct:username@domain'")
	}

	if !strings.Contains(parts[1], "@") {
		return "", errors.New(errors.ErrBadRequest, invalidResourceFormatMessage)
	}

	uparts := strings.SplitN(parts[1], "@", 2)
	if len(uparts) != 2 {
		return "", errors.New(errors.ErrBadRequest, invalidResourceFormatMessage)
	}

	username, domain := uparts[0], uparts[1]
	if username == "" || domain == "" {
		return "", errors.New(errors.ErrBadRequest, "username or domain cannot be empty")
	}

	if h.cfg.Domain != domain {
		return "", errors.New(errors.ErrBadRequest, fmt.Sprintf("domain mismatch, expected '%s', got '%s'", h.cfg.Domain, domain))
	}

	exists, err := h.cfg.UsersRepo.UserWithUsernameExists(ctx, nil, username)
	if err != nil {
		return "", errors.NewErr(errors.ErrInternal, err)
	}

	if !exists {
		return "", errors.New(errors.ErrNotFound, "username does not exist")
	}

	return fmt.Sprintf(`{
  "subject": "acct:%s@%s",
  "links": [
    {
      "rel": "self",
      "type": "application/activity+json",
      "href": "%s/users/%s"
    }
  ]
}`, username, h.cfg.Domain, h.cfg.Host, username), nil
}
