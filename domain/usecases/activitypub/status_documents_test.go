package activitypub

import (
	"testing"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
)

func TestStatusUpdateObjectIncludesUpdatedTimestamp(t *testing.T) {
	editedAt := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	doc := StatusUpdateObject(models.Account{URI: "https://example.test/users/alice"}, models.Note{URI: "https://example.test/notes/1", ObjectType: "Note", Content: "edited", EditedAt: &editedAt})
	if got := doc["updated"]; got != "2026-06-08T12:00:00Z" {
		t.Fatalf("updated = %#v, want RFC3339 edited timestamp", got)
	}
}
