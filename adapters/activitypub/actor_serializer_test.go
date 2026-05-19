package activitypub

import (
	"encoding/json"
	"testing"

	"github.com/myfedi/gargoyle/domain/models"
)

func TestActorSerializerMarshallUsesAccountURI(t *testing.T) {
	serializer := NewActorSerializer(ActorSerializerConfig{})
	actorJSON, err := serializer.Marshall(models.Account{
		Username:     "alice",
		URI:          "https://example.org/users/alice",
		InboxURI:     "https://example.org/users/alice/inbox",
		OutboxURI:    strPtr("https://example.org/users/alice/outbox"),
		FollowersURI: "https://example.org/users/alice/followers",
		FollowingURI: "https://example.org/users/alice/following",
		PublicKey:    "public-key-pem",
		ActorType:    models.ActorTypePerson,
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
	if got["@context"] != "https://www.w3.org/ns/activitystreams" {
		t.Fatalf("expected ActivityStreams context, got %v", got["@context"])
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
