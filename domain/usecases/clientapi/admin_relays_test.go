package clientapi

import (
	"encoding/json"
	"testing"

	"github.com/myfedi/gargoyle/domain/models"
)

func TestNormalizeRelayActorRequiresHTTPSActorURL(t *testing.T) {
	if _, derr := normalizeRelayActor("http://relay.example/actor"); derr == nil {
		t.Fatal("expected http relay actor to be rejected")
	}
	got, derr := normalizeRelayActor("https://relay.example/actor#fragment")
	if derr != nil {
		t.Fatalf("normalizeRelayActor returned error: %v", derr)
	}
	if got != "https://relay.example/actor" {
		t.Fatalf("normalized relay actor = %q", got)
	}
}

func TestRelayFollowPayloadTargetsPublicCollection(t *testing.T) {
	account := models.Account{URI: "https://gargoyle.example/users/alice"}
	raw, err := marshalRelayFollow(account, "01TEST")
	if err != nil {
		t.Fatalf("marshalRelayFollow returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal follow: %v", err)
	}
	if got["type"] != "Follow" || got["actor"] != account.URI || got["object"] != activityStreamsPublicURI {
		t.Fatalf("unexpected relay Follow payload: %s", raw)
	}
}

func TestDisabledRelaysRejectAdminOperations(t *testing.T) {
	u := Moderation{deps: ModerationConfig{RelaysEnabled: false}}
	if derr := u.requireRelaysEnabled(); derr == nil {
		t.Fatal("expected disabled relay feature to reject operation")
	}
}
