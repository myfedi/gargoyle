package activitypub

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// CacheRemoteOutboxPage fetches one remote actor outbox collection/page, hydrates
// Create/Note/Announce items from that page, and returns the next page URL.
func (u HydrateRemoteObjectUseCase) CacheRemoteOutboxPage(ctx context.Context, account models.Account, pageURI, expectedActor string, shouldStop func() (bool, error)) (string, error) {
	if pageURI == "" {
		return "", nil
	}
	raw, err := u.fetcher.FetchObject(ctx, pageURI, &account)
	if err != nil {
		return "", err
	}
	items, next := outboxItems(raw)
	if len(items) == 0 && next == "" && u.outboxResolver != nil {
		resolvedItems, resolvedNext, err := u.outboxResolver.ResolveOutboxPage(ctx, account, pageURI, expectedActor)
		if err == nil && (len(resolvedItems) > 0 || resolvedNext != "") {
			items, next = resolvedItems, resolvedNext
		}
	}
	for index, item := range items {
		if shouldStop != nil && index%5 == 0 {
			stop, err := shouldStop()
			if err != nil || stop {
				return next, err
			}
		}
		_ = u.cacheRemoteOutboxItem(ctx, account, expectedActor, item)
	}
	if shouldStop != nil {
		stop, err := shouldStop()
		if err != nil || stop {
			return next, err
		}
	}
	return next, nil
}

func (u HydrateRemoteObjectUseCase) cacheRemoteOutboxItem(ctx context.Context, account models.Account, expectedActor string, raw json.RawMessage) error {
	var itemURL string
	if err := json.Unmarshal(raw, &itemURL); err == nil && itemURL != "" {
		fetched, err := u.fetcher.FetchObject(ctx, itemURL, &account)
		if err != nil {
			return err
		}
		raw = fetched
	}
	if createObject, ok := extractOutboxCreateObject(raw); ok {
		objectRaw, note, err := u.outboxCreateNote(ctx, account, createObject)
		if err != nil {
			return err
		}
		objectURI := note.URI
		if objectURI == "" {
			objectURI = createObject.Object
		}
		if len(objectRaw) == 0 || objectURI == "" {
			return nil
		}
		if expectedActor != "" && (createObject.Actor == "" || createObject.Actor != expectedActor || note.AttributedTo == "" || note.AttributedTo != expectedActor) {
			_ = u.hydrateRawObject(ctx, account, objectRaw, "", false)
			published := createObject.Published
			if published.IsZero() {
				published = note.PublishedAt
			}
			return u.createRemoteOutboxBoost(ctx, account, expectedActor, objectURI, createObject.ID, published)
		}
		raw = objectRaw
	}
	if announce, ok := extractOutboxAnnounce(raw); ok {
		return u.hydrateRemoteAnnounce(ctx, account, expectedActor, announce)
	}
	return u.hydrateRawObject(ctx, account, raw, expectedActor, false)
}

type outboxCreateObject struct {
	ID        string
	Actor     string
	Object    string
	ObjectRaw json.RawMessage
	Published time.Time
}

func extractOutboxCreateObject(raw []byte) (outboxCreateObject, bool) {
	var doc struct {
		ID        string          `json:"id"`
		Type      string          `json:"type"`
		Actor     json.RawMessage `json:"actor"`
		Object    json.RawMessage `json:"object"`
		Published string          `json:"published"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Type != "Create" || len(doc.Object) == 0 {
		return outboxCreateObject{}, false
	}
	actor, _, _ := ExtractIDAndInbox(doc.Actor)
	published, _ := time.Parse(time.RFC3339, doc.Published)
	var objectURL string
	if err := json.Unmarshal(doc.Object, &objectURL); err == nil && objectURL != "" {
		return outboxCreateObject{ID: doc.ID, Actor: actor, Object: objectURL, Published: published}, true
	}
	object, _, err := ExtractIDAndInbox(doc.Object)
	if err != nil {
		return outboxCreateObject{}, false
	}
	return outboxCreateObject{ID: doc.ID, Actor: actor, Object: object, ObjectRaw: doc.Object, Published: published}, true
}

type outboxAnnounce struct {
	ID        string
	Actor     string
	Object    string
	ObjectRaw json.RawMessage
	Published time.Time
}

func (u HydrateRemoteObjectUseCase) outboxCreateNote(ctx context.Context, account models.Account, createObject outboxCreateObject) ([]byte, ExtractedNote, error) {
	if len(createObject.ObjectRaw) > 0 {
		note, _ := extractFetchedNote(createObject.ObjectRaw)
		return createObject.ObjectRaw, note, nil
	}
	if createObject.Object == "" {
		return nil, ExtractedNote{}, nil
	}
	fetched, err := u.fetcher.FetchObject(ctx, createObject.Object, &account)
	if err != nil {
		return nil, ExtractedNote{}, err
	}
	note, _ := extractFetchedNote(fetched)
	return fetched, note, nil
}

func (u HydrateRemoteObjectUseCase) createRemoteOutboxBoost(ctx context.Context, account models.Account, actor, objectURI, activityID string, published time.Time) error {
	if u.boosts == nil || actor == "" || objectURI == "" {
		return nil
	}
	persist := func(ctx context.Context, tx *db.Tx) error {
		note, err := u.notes.GetNoteByURI(ctx, tx, objectURI)
		if err != nil {
			return nil
		}
		uri := activityID
		if uri == "" {
			uri = objectURI + "#outbox-" + actor
		}
		if published.IsZero() {
			published = time.Now().UTC()
		}
		_, err = u.boosts.CreateBoost(ctx, tx, repos.CreateBoostInput{LocalAccountID: account.ID, Actor: actor, NoteID: note.ID, URI: uri, PublishedAt: published})
		return err
	}
	if u.txProvider == nil {
		return persist(ctx, nil)
	}
	return u.txProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		return persist(ctx, &tx)
	})
}

func extractOutboxAnnounce(raw []byte) (outboxAnnounce, bool) {
	var doc struct {
		ID        string          `json:"id"`
		Type      string          `json:"type"`
		Actor     json.RawMessage `json:"actor"`
		Object    json.RawMessage `json:"object"`
		Published string          `json:"published"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Type != "Announce" || len(doc.Actor) == 0 || len(doc.Object) == 0 {
		return outboxAnnounce{}, false
	}
	actor, _, err := ExtractIDAndInbox(doc.Actor)
	if err != nil || actor == "" {
		return outboxAnnounce{}, false
	}
	object, _, err := ExtractIDAndInbox(doc.Object)
	if err != nil {
		return outboxAnnounce{}, false
	}
	published, _ := time.Parse(time.RFC3339, doc.Published)
	return outboxAnnounce{ID: doc.ID, Actor: actor, Object: object, ObjectRaw: doc.Object, Published: published}, true
}

func (u HydrateRemoteObjectUseCase) hydrateRemoteAnnounce(ctx context.Context, account models.Account, expectedActor string, announce outboxAnnounce) error {
	if u.boosts == nil || announce.Object == "" || announce.Actor == "" {
		return nil
	}
	if expectedActor != "" && announce.Actor != expectedActor {
		return nil
	}
	noteURI := announce.Object
	if createObject, ok := extractOutboxCreateObject(announce.ObjectRaw); ok {
		objectRaw, note, err := u.outboxCreateNote(ctx, account, createObject)
		if err != nil {
			return err
		}
		if note.URI != "" {
			noteURI = note.URI
		} else if createObject.Object != "" {
			noteURI = createObject.Object
		}
		if announce.Published.IsZero() {
			announce.Published = createObject.Published
		}
		if announce.Published.IsZero() {
			announce.Published = note.PublishedAt
		}
		if len(objectRaw) > 0 {
			_ = u.hydrateRawObject(ctx, account, objectRaw, "", false)
		}
	} else if _, err := u.notes.GetNoteByURI(ctx, nil, noteURI); err != nil {
		var embedded map[string]any
		if err := json.Unmarshal(announce.ObjectRaw, &embedded); err == nil {
			if raw, err := json.Marshal(embedded); err == nil {
				_ = u.hydrateRawObject(ctx, account, raw, "", false)
			}
		} else if fetched, err := u.fetcher.FetchObject(ctx, announce.Object, &account); err == nil {
			_ = u.hydrateRawObject(ctx, account, fetched, "", false)
		}
	}
	if noteURI == "" {
		return nil
	}
	persist := func(ctx context.Context, tx *db.Tx) error {
		note, err := u.notes.GetNoteByURI(ctx, tx, noteURI)
		if err != nil {
			return nil
		}
		uri := announce.ID
		if uri == "" {
			uri = noteURI + "#announce-" + announce.Actor
		}
		published := announce.Published
		if published.IsZero() {
			published = note.PublishedAt
		}
		if published.IsZero() {
			published = time.Now().UTC()
		}
		_, err = u.boosts.CreateBoost(ctx, tx, repos.CreateBoostInput{LocalAccountID: account.ID, Actor: announce.Actor, NoteID: note.ID, URI: uri, PublishedAt: published})
		return err
	}
	if u.txProvider == nil {
		return persist(ctx, nil)
	}
	return u.txProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		return persist(ctx, &tx)
	})
}

func outboxItems(raw []byte) ([]json.RawMessage, string) {
	var doc struct {
		Type         string            `json:"type"`
		First        json.RawMessage   `json:"first"`
		Next         json.RawMessage   `json:"next"`
		OrderedItems []json.RawMessage `json:"orderedItems"`
		Items        []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, ""
	}
	items := doc.OrderedItems
	if len(items) == 0 {
		items = doc.Items
	}
	next := collectionRef(doc.Next)
	if len(items) > 0 || next != "" {
		return items, next
	}
	if first := collectionRef(doc.First); first != "" {
		return nil, first
	}
	if len(doc.First) > 0 {
		return outboxItems(doc.First)
	}
	return nil, ""
}

func collectionRef(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.ID
	}
	return ""
}
