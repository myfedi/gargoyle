package activitypub

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
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

func TestInboxAcceptPublishesRelationshipUpdate(t *testing.T) {
	app := fiber.New()
	remoteActor := "https://remote.example/users/bob"
	follows := &fakeFollowsRepo{followers: []models.Follow{{ID: "following", LocalAccountID: "account-1", RemoteActor: remoteActor, Direction: "following"}}}
	activities := &fakeActivitiesRepo{}
	var gotLocal, gotActor string
	var gotNotifications bool
	handler := newTestHandler(fakeAccountsRepo{}, activities, follows)
	handler.SetRealtimePublisher(func(localAccountID, remoteActor string, notifications bool) {
		gotLocal = localAccountID
		gotActor = remoteActor
		gotNotifications = notifications
	})
	handler.SetupRoutes(app)

	body := `{"type":"Accept","actor":"https://remote.example/users/bob","object":{"type":"Follow","actor":"https://example.org/users/alice","object":"https://remote.example/users/bob"}}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if gotLocal != "account-1" || gotActor != remoteActor {
		t.Fatalf("unexpected realtime publish local=%q actor=%q", gotLocal, gotActor)
	}
	if gotNotifications {
		t.Fatal("Accept should publish relationship update without notification update")
	}
}
