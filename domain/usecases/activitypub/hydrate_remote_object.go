package activitypub

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// HydrateRemoteObjectUseCase fetches and persists missing remote ActivityPub
// objects, primarily reply parents discovered through inReplyTo links.
type HydrateRemoteObjectUseCase struct {
	fetcher    ports.RemoteObjectFetcher
	activities repos.ActivitiesRepository
	notes      repos.NotesRepository
	sanitizer  ports.ContentSanitizer
}

type HydrateRemoteObjectConfig struct {
	Fetcher        ports.RemoteObjectFetcher
	ActivitiesRepo repos.ActivitiesRepository
	NotesRepo      repos.NotesRepository
	Sanitizer      ports.ContentSanitizer
}

func NewHydrateRemoteObjectUseCase(cfg HydrateRemoteObjectConfig) HydrateRemoteObjectUseCase {
	if cfg.Fetcher == nil {
		panic("hydrate remote object use case requires Fetcher")
	}
	if cfg.ActivitiesRepo == nil {
		panic("hydrate remote object use case requires ActivitiesRepo")
	}
	if cfg.NotesRepo == nil {
		panic("hydrate remote object use case requires NotesRepo")
	}
	if cfg.Sanitizer == nil {
		panic("hydrate remote object use case requires Sanitizer")
	}
	return HydrateRemoteObjectUseCase{fetcher: cfg.Fetcher, activities: cfg.ActivitiesRepo, notes: cfg.NotesRepo, sanitizer: cfg.Sanitizer}
}

func (u HydrateRemoteObjectUseCase) HydrateRemoteObject(ctx context.Context, account models.Account, objectURI string) error {
	if _, err := u.notes.GetNoteByURI(ctx, nil, objectURI); err == nil {
		return nil
	}
	raw, err := u.fetcher.FetchObject(ctx, objectURI, &account)
	if err != nil {
		return err
	}
	note, ok := extractFetchedNote(raw)
	if !ok {
		return nil
	}
	if _, err := u.notes.GetNoteByURI(ctx, nil, note.URI); err == nil {
		return nil
	} else if err != sql.ErrNoRows {
		return err
	}
	activity, err := u.activities.CreateActivity(ctx, nil, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionInbox, Type: "Create", Actor: note.AttributedTo, Object: note.URI, RawJSON: string(raw)})
	if err != nil {
		return err
	}
	replyID, replyURI := replyIDs(ctx, u.notes, nil, note)
	_, err = u.notes.CreateNote(ctx, nil, repos.CreateNoteInput{LocalAccountID: account.ID, ActivityID: activity.ID, URI: note.URI, Content: u.sanitizer.SanitizeHTML(note.Content), PlainText: u.sanitizer.StripHTMLFromText(note.Content), ObjectType: note.Type, PollMultiple: note.PollMultiple, PollExpiresAt: note.PollExpiresAt, Visibility: note.Visibility, Sensitive: note.Sensitive, SpoilerText: note.SpoilerText, AttributedTo: note.AttributedTo, InReplyToID: replyID, InReplyToURI: replyURI, PublishedAt: note.PublishedAt})
	return err
}

func extractFetchedNote(raw []byte) (ExtractedNote, bool) {
	if note, ok := ExtractNote(raw); ok {
		return note, true
	}
	var doc struct {
		ID           string  `json:"id"`
		Type         string  `json:"type"`
		Content      string  `json:"content"`
		AttributedTo string  `json:"attributedTo"`
		InReplyTo    *string `json:"inReplyTo"`
		Published    string  `json:"published"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Type != "Note" || doc.ID == "" {
		return ExtractedNote{}, false
	}
	publishedAt, err := time.Parse(time.RFC3339, doc.Published)
	if err != nil {
		publishedAt = time.Now().UTC()
	}
	return ExtractedNote{URI: doc.ID, Content: doc.Content, AttributedTo: doc.AttributedTo, InReplyToURI: doc.InReplyTo, PublishedAt: publishedAt}, true
}
