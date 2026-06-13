package activitypub

import "testing"

func TestOutboxItemsFollowsStringFirstCollection(t *testing.T) {
	raw := []byte(`{
		"type":"OrderedCollection",
		"first":"https://remote.example/users/alice/outbox?page=true"
	}`)

	items, next := outboxItems(raw)
	if len(items) != 0 {
		t.Fatalf("items len = %d, want 0", len(items))
	}
	if next != "https://remote.example/users/alice/outbox?page=true" {
		t.Fatalf("next = %q", next)
	}
}

func TestOutboxItemsReadsEmbeddedFirstPage(t *testing.T) {
	raw := []byte(`{
		"type":"OrderedCollection",
		"first":{
			"type":"OrderedCollectionPage",
			"next":"https://remote.example/users/alice/outbox?page=2",
			"orderedItems":[
				{"id":"https://remote.example/activities/1","type":"Create"}
			]
		}
	}`)

	items, next := outboxItems(raw)
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if next != "https://remote.example/users/alice/outbox?page=2" {
		t.Fatalf("next = %q", next)
	}
}

func TestExtractOutboxCreateObjectWithObjectURL(t *testing.T) {
	raw := []byte(`{
		"type":"Create",
		"actor":"https://remote.example/users/alice",
		"object":"https://remote.example/users/alice/statuses/1"
	}`)

	create, ok := extractOutboxCreateObject(raw)
	if !ok {
		t.Fatal("expected Create object URL")
	}
	if create.Actor != "https://remote.example/users/alice" {
		t.Fatalf("actor = %q", create.Actor)
	}
	if create.Object != "https://remote.example/users/alice/statuses/1" {
		t.Fatalf("object = %q", create.Object)
	}
	if len(create.ObjectRaw) != 0 {
		t.Fatalf("ObjectRaw len = %d, want 0", len(create.ObjectRaw))
	}
}
