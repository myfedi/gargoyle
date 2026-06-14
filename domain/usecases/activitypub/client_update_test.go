package activitypub

import "testing"

func TestClientUpdateForInboxActivityRelationshipOnly(t *testing.T) {
	update := clientUpdateForInboxActivity("account-1", ParsedActivity{Type: "Accept", Actor: "https://remote.example/users/bob"})
	if update == nil {
		t.Fatal("expected client update")
	}
	if update.LocalAccountID != "account-1" {
		t.Fatalf("LocalAccountID = %q", update.LocalAccountID)
	}
	if update.RelationshipActorURI != "https://remote.example/users/bob" {
		t.Fatalf("RelationshipActorURI = %q", update.RelationshipActorURI)
	}
	if update.NotificationsChanged {
		t.Fatal("Accept should not mark notifications changed")
	}
}

func TestClientUpdateForInboxActivityNotification(t *testing.T) {
	update := clientUpdateForInboxActivity("account-1", ParsedActivity{Type: "Follow", Actor: "https://remote.example/users/bob"})
	if update == nil {
		t.Fatal("expected client update")
	}
	if update.RelationshipActorURI != "https://remote.example/users/bob" {
		t.Fatalf("RelationshipActorURI = %q", update.RelationshipActorURI)
	}
	if !update.NotificationsChanged {
		t.Fatal("Follow should mark notifications changed")
	}
}

func TestClientUpdateForInboxActivitySkipsUninterestingActivity(t *testing.T) {
	if update := clientUpdateForInboxActivity("account-1", ParsedActivity{Type: "Update", Actor: "https://remote.example/users/bob"}); update != nil {
		t.Fatalf("expected no update, got %#v", update)
	}
}
