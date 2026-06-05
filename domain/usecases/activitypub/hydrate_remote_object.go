package activitypub

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// HydrateRemoteObjectUseCase fetches and persists missing remote ActivityPub
// objects, primarily reply parents discovered through inReplyTo links.
type HydrateRemoteObjectUseCase struct {
	fetcher    ports.RemoteObjectFetcher
	txProvider db.TxProvider
	activities repos.ActivitiesRepository
	notes      repos.NotesRepository
	sanitizer  ports.ContentSanitizer
}

type HydrateRemoteObjectConfig struct {
	Fetcher        ports.RemoteObjectFetcher
	TxProvider     db.TxProvider
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
	return HydrateRemoteObjectUseCase{fetcher: cfg.Fetcher, txProvider: cfg.TxProvider, activities: cfg.ActivitiesRepo, notes: cfg.NotesRepo, sanitizer: cfg.Sanitizer}
}

func (u HydrateRemoteObjectUseCase) HydrateRemoteObject(ctx context.Context, account models.Account, objectURI string) error {
	if _, err := u.notes.GetNoteByURI(ctx, nil, objectURI); err == nil {
		return nil
	}
	raw, err := u.fetcher.FetchObject(ctx, objectURI, &account)
	if err != nil {
		return err
	}
	return u.HydrateRawObject(ctx, account, raw, "")
}

// HydrateRawObject persists a fetched ActivityPub Create/Note document. If expectedActor
// is set, the extracted note must be attributed to that actor.
func (u HydrateRemoteObjectUseCase) HydrateRawObject(ctx context.Context, account models.Account, raw []byte, expectedActor string) error {
	note, ok := extractFetchedNote(raw)
	if !ok || note.URI == "" || note.Visibility == "direct" {
		return nil
	}
	if expectedActor != "" && note.AttributedTo != expectedActor {
		return nil
	}
	persist := func(ctx context.Context, tx *db.Tx) error {
		if _, err := u.notes.GetNoteByURI(ctx, tx, note.URI); err == nil {
			return nil
		} else if err != sql.ErrNoRows {
			return err
		}
		activity, err := u.activities.CreateActivity(ctx, tx, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionInbox, Type: "Create", Actor: note.AttributedTo, Object: note.URI, RawJSON: string(raw)})
		if err != nil {
			return err
		}
		replyID, replyURI := replyIDs(ctx, u.notes, tx, note)
		publishedAt := note.PublishedAt
		if publishedAt.IsZero() {
			publishedAt = time.Now().UTC()
		}
		_, err = u.notes.CreateNote(ctx, tx, repos.CreateNoteInput{LocalAccountID: account.ID, ActivityID: activity.ID, URI: note.URI, Content: u.sanitizer.SanitizeHTML(note.Content), PlainText: u.sanitizer.StripHTMLFromText(note.Content), ObjectType: note.Type, PollMultiple: note.PollMultiple, PollExpiresAt: note.PollExpiresAt, Hashtags: note.Hashtags, Emojis: note.Emojis, Visibility: note.Visibility, Sensitive: note.Sensitive, SpoilerText: note.SpoilerText, AttributedTo: note.AttributedTo, InReplyToID: replyID, InReplyToURI: replyURI, PublishedAt: publishedAt})
		return err
	}
	if u.txProvider == nil {
		return persist(ctx, nil)
	}
	return u.txProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		return persist(ctx, &tx)
	})
}

func extractFetchedNote(raw []byte) (ExtractedNote, bool) {
	if note, ok := ExtractNote(raw); ok {
		return note, true
	}
	if note, ok := ExtractStandaloneNote(raw); ok {
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
	return ExtractedNote{URI: doc.ID, Type: doc.Type, Content: doc.Content, AttributedTo: doc.AttributedTo, InReplyToURI: doc.InReplyTo, PublishedAt: publishedAt}, true
}
