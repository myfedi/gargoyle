package activitypub

import (
	"context"
	"testing"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type relaySubscriptionsRepoStub struct {
	accepted []models.RelaySubscription
	listed   bool
}

func (r *relaySubscriptionsRepoStub) CreateRelaySubscription(context.Context, *db.Tx, repos.CreateRelaySubscriptionInput) (*models.RelaySubscription, error) {
	return nil, nil
}
func (r *relaySubscriptionsRepoStub) ListRelaySubscriptions(context.Context, *db.Tx) ([]models.RelaySubscription, error) {
	return nil, nil
}
func (r *relaySubscriptionsRepoStub) ListAcceptedRelaySubscriptions(context.Context, *db.Tx) ([]models.RelaySubscription, error) {
	r.listed = true
	return r.accepted, nil
}
func (r *relaySubscriptionsRepoStub) GetRelaySubscriptionByActor(context.Context, *db.Tx, string) (*models.RelaySubscription, error) {
	return nil, nil
}
func (r *relaySubscriptionsRepoStub) GetRelaySubscriptionByID(context.Context, *db.Tx, string) (*models.RelaySubscription, error) {
	return nil, nil
}
func (r *relaySubscriptionsRepoStub) MarkRelaySubscriptionAccepted(context.Context, *db.Tx, string, time.Time) error {
	return nil
}
func (r *relaySubscriptionsRepoStub) DisableRelaySubscription(context.Context, *db.Tx, string) error {
	return nil
}
func (r *relaySubscriptionsRepoStub) DeleteRelaySubscription(context.Context, *db.Tx, string) error {
	return nil
}

func TestRelayInboxesRequireEnabledPublicNote(t *testing.T) {
	repo := &relaySubscriptionsRepoStub{accepted: []models.RelaySubscription{{InboxURI: "https://relay.example/inbox"}}}
	u := &CreateOutboxActivityUseCase{cfg: ActivityPubFlowConfig{RelaysRepo: repo}}
	inboxes, derr := u.relayInboxes(context.Background(), &models.Note{Visibility: "public"})
	if derr != nil {
		t.Fatalf("relayInboxes returned error: %v", derr)
	}
	if repo.listed || len(inboxes) != 0 {
		t.Fatalf("disabled relays should not be listed or returned, listed=%v inboxes=%v", repo.listed, inboxes)
	}

	u.cfg.RelaysEnabled = true
	inboxes, derr = u.relayInboxes(context.Background(), &models.Note{Visibility: "private"})
	if derr != nil {
		t.Fatalf("relayInboxes returned error: %v", derr)
	}
	if repo.listed || len(inboxes) != 0 {
		t.Fatalf("non-public notes should not be delivered to relays, listed=%v inboxes=%v", repo.listed, inboxes)
	}

	inboxes, derr = u.relayInboxes(context.Background(), &models.Note{Visibility: "public"})
	if derr != nil {
		t.Fatalf("relayInboxes returned error: %v", derr)
	}
	if len(inboxes) != 1 || inboxes[0] != "https://relay.example/inbox" {
		t.Fatalf("relay inboxes = %#v", inboxes)
	}
}

func TestIsRelayFollowAccept(t *testing.T) {
	raw := []byte(`{"type":"Accept","actor":"https://relay.example/actor","object":{"type":"Follow","actor":"https://gargoyle.example/users/alice","object":"https://www.w3.org/ns/activitystreams#Public"}}`)
	if !isRelayFollowAccept(raw, "https://gargoyle.example/users/alice") {
		t.Fatal("expected relay follow Accept to be recognized")
	}
	if isRelayFollowAccept(raw, "https://gargoyle.example/users/bob") {
		t.Fatal("wrong local actor should not match relay follow Accept")
	}
}
