package activitypub

import (
	"encoding/json"
	"testing"

	"github.com/myfedi/gargoyle/domain/models"
)

func TestActorSerializerMarshallUsesAccountURI(t *testing.T) {
	serializer := NewActorSerializer(ActorSerializerConfig{})
	actorJSON, err := serializer.Marshall(models.Account{
		Username:              "alice",
		DisplayName:           strPtr("Alice A."),
		AvatarMediaID:         strPtr("avatar-1"),
		HeaderURL:             strPtr("https://cdn.example.org/header.png"),
		URI:                   "https://example.org/users/alice",
		InboxURI:              "https://example.org/users/alice/inbox",
		OutboxURI:             strPtr("https://example.org/users/alice/outbox"),
		FollowersURI:          "https://example.org/users/alice/followers",
		FollowingURI:          "https://example.org/users/alice/following",
		FeaturedCollectionURI: "https://example.org/users/alice/collections/featured",
		PublicKey:             "public-key-pem",
		ActorType:             models.ActorTypePerson,
	})
	if err != nil {
		t.Fatalf("Marshall returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(actorJSON), &got); err != nil {
		t.Fatalf("actor JSON is invalid: %v", err)
	}

	if got["id"] != "https://example.org/users/alice" {
		t.Fatalf("expected id to use account URI, got %v", got["id"])
	}
	if got["type"] != "Person" {
		t.Fatalf("expected Person actor, got %v", got["type"])
	}
	if got[activityStreamsContextKey] != activityStreamsContextURI {
		t.Fatalf("expected ActivityStreams context, got %v", got[activityStreamsContextKey])
	}
	if got["name"] != "Alice A." {
		t.Fatalf("expected display name in actor name, got %v", got["name"])
	}
	if got["featured"] != "https://example.org/users/alice/collections/featured" {
		t.Fatalf("expected featured collection, got %v", got["featured"])
	}
	endpoints, ok := got["endpoints"].(map[string]any)
	if !ok || endpoints["sharedInbox"] != "https://example.org/inbox" {
		t.Fatalf("expected shared inbox endpoint, got %#v", got["endpoints"])
	}
	icon, ok := got["icon"].(map[string]any)
	if !ok || icon["url"] != "https://example.org/media/avatar-1" {
		t.Fatalf("expected local avatar icon URL, got %#v", got["icon"])
	}
	image, ok := got["image"].(map[string]any)
	if !ok || image["url"] != "https://cdn.example.org/header.png" {
		t.Fatalf("expected header image URL, got %#v", got["image"])
	}
}

func TestActorSerializerMarshallRejectsUnknownActorType(t *testing.T) {
	serializer := NewActorSerializer(ActorSerializerConfig{})
	_, err := serializer.Marshall(models.Account{ActorType: models.ActorTypeUnknown})
	if err == nil {
		t.Fatal("expected error for unknown actor type")
	}
}

func strPtr(s string) *string { return &s }
