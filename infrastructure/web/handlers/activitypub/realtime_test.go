package activitypub

import (
	"testing"

	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

func TestPublishRealtimeDispatchesClientUpdate(t *testing.T) {
	var gotLocal, gotActor string
	var gotNotifications bool
	h := &Handler{cfg: HandlerConfig{RealtimePublisher: func(localAccountID, remoteActor string, notifications bool) {
		gotLocal = localAccountID
		gotActor = remoteActor
		gotNotifications = notifications
	}}}

	h.publishRealtime(&apUsecases.HandleInboxActivityResult{ClientUpdate: &apUsecases.ClientUpdate{LocalAccountID: "account-1", RelationshipActorURI: "https://remote.example/users/bob", NotificationsChanged: true}})

	if gotLocal != "account-1" || gotActor != "https://remote.example/users/bob" || !gotNotifications {
		t.Fatalf("unexpected publish values local=%q actor=%q notifications=%v", gotLocal, gotActor, gotNotifications)
	}
}

func TestPublishRealtimeSkipsMissingClientUpdate(t *testing.T) {
	called := false
	h := &Handler{cfg: HandlerConfig{RealtimePublisher: func(string, string, bool) { called = true }}}

	h.publishRealtime(&apUsecases.HandleInboxActivityResult{})

	if called {
		t.Fatal("publisher should not be called without ClientUpdate")
	}
}
