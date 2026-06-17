package clientapi

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

const activityStreamsPublicURI = "https://www.w3.org/ns/activitystreams#Public"

type AddRelayInput struct {
	ActorURI string
	FollowID string
}

type AddRelayResult struct {
	Relay   models.RelaySubscription
	Account models.Account
	RawJSON []byte
	Inbox   string
}

type DisableRelayResult struct {
	Relay   models.RelaySubscription
	Account models.Account
	RawJSON []byte
	Inbox   string
}

func (u Moderation) ListRelays(ctx context.Context, admin *models.User) ([]models.RelaySubscription, *domainerrors.DomainError) {
	if derr := requireAdmin(admin); derr != nil {
		return nil, derr
	}
	if derr := u.requireRelaysEnabled(); derr != nil {
		return nil, derr
	}
	if u.deps.RelaysRepo == nil {
		return nil, domainerrors.New(domainerrors.ErrInternal, "relay repository is not configured")
	}
	relays, err := u.deps.RelaysRepo.ListRelaySubscriptions(ctx, nil)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return relays, nil
}

func (u Moderation) AddRelay(ctx context.Context, admin *models.User, account *models.Account, input AddRelayInput) (*AddRelayResult, *domainerrors.DomainError) {
	if derr := requireAdmin(admin); derr != nil {
		return nil, derr
	}
	if account == nil {
		return nil, domainerrors.New(domainerrors.ErrUnauthorized, "admin account is required")
	}
	if derr := u.requireRelaysEnabled(); derr != nil {
		return nil, derr
	}
	if u.deps.RelaysRepo == nil || u.deps.ActorFetcher == nil {
		return nil, domainerrors.New(domainerrors.ErrInternal, "relay dependencies are not configured")
	}
	actorURI, derr := normalizeRelayActor(input.ActorURI)
	if derr != nil {
		return nil, derr
	}
	if strings.TrimSpace(input.FollowID) == "" {
		return nil, domainerrors.New(domainerrors.ErrInternal, "follow id is required")
	}
	actor, err := u.deps.ActorFetcher.FetchActor(ctx, actorURI, account)
	if err != nil || actor == nil || actor.Inbox == "" {
		if err == nil {
			err = domainerrors.New(domainerrors.ErrBadRequest, "relay actor does not advertise an inbox")
		}
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	relay, err := u.deps.RelaysRepo.CreateRelaySubscription(ctx, nil, repos.CreateRelaySubscriptionInput{ActorURI: actorURI, InboxURI: actor.Inbox, CreatedByUserID: admin.ID})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	raw, err := marshalRelayFollow(*account, input.FollowID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &AddRelayResult{Relay: *relay, Account: *account, RawJSON: raw, Inbox: actor.Inbox}, nil
}

func (u Moderation) DisableRelay(ctx context.Context, admin *models.User, account *models.Account, relayID, undoID string) (*DisableRelayResult, *domainerrors.DomainError) {
	if derr := requireAdmin(admin); derr != nil {
		return nil, derr
	}
	if account == nil {
		return nil, domainerrors.New(domainerrors.ErrUnauthorized, "admin account is required")
	}
	if derr := u.requireRelaysEnabled(); derr != nil {
		return nil, derr
	}
	if u.deps.RelaysRepo == nil {
		return nil, domainerrors.New(domainerrors.ErrInternal, "relay repository is not configured")
	}
	relay, err := u.deps.RelaysRepo.GetRelaySubscriptionByID(ctx, nil, strings.TrimSpace(relayID))
	if err != nil {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "relay not found")
	}
	if err := u.deps.RelaysRepo.DisableRelaySubscription(ctx, nil, relay.ActorURI); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	relay.Status = models.RelayStatusDisabled
	relay.UpdatedAt = time.Now().UTC()
	raw, err := marshalRelayUndo(*account, undoID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &DisableRelayResult{Relay: *relay, Account: *account, RawJSON: raw, Inbox: relay.InboxURI}, nil
}

func (u Moderation) DeleteRelay(ctx context.Context, admin *models.User, relayID string) *domainerrors.DomainError {
	if derr := requireAdmin(admin); derr != nil {
		return derr
	}
	if derr := u.requireRelaysEnabled(); derr != nil {
		return derr
	}
	if u.deps.RelaysRepo == nil {
		return domainerrors.New(domainerrors.ErrInternal, "relay repository is not configured")
	}
	relay, err := u.deps.RelaysRepo.GetRelaySubscriptionByID(ctx, nil, strings.TrimSpace(relayID))
	if err != nil {
		return domainerrors.New(domainerrors.ErrNotFound, "relay not found")
	}
	if err := u.deps.RelaysRepo.DeleteRelaySubscription(ctx, nil, relay.ActorURI); err != nil {
		return domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return nil
}

func (u Moderation) requireRelaysEnabled() *domainerrors.DomainError {
	if !u.deps.RelaysEnabled {
		return domainerrors.New(domainerrors.ErrBadRequest, "ActivityPub relays are disabled")
	}
	return nil
}

func marshalRelayFollow(account models.Account, id string) ([]byte, error) {
	return json.Marshal(map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": account.URI + "/relay-follows/" + id, "type": "Follow", "actor": account.URI, "object": activityStreamsPublicURI})
}

func marshalRelayUndo(account models.Account, id string) ([]byte, error) {
	return json.Marshal(map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": account.URI + "/relay-undos/" + id, "type": "Undo", "actor": account.URI, "object": map[string]any{"type": "Follow", "actor": account.URI, "object": activityStreamsPublicURI}})
}

func normalizeRelayActor(raw string) (string, *domainerrors.DomainError) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", domainerrors.New(domainerrors.ErrBadRequest, "relay actor is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return "", domainerrors.New(domainerrors.ErrBadRequest, "relay actor must be an https URL")
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}
