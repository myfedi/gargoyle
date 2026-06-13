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
	for _, item := range items {
		if shouldStop != nil {
			stop, err := shouldStop()
			if err != nil || stop {
				return next, err
			}
		}
		_ = u.cacheRemoteOutboxItem(ctx, account, expectedActor, item)
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
		if expectedActor != "" && createObject.Actor != "" && createObject.Actor != expectedActor {
			return nil
		}
		if len(createObject.ObjectRaw) > 0 {
			raw = createObject.ObjectRaw
		} else if createObject.Object != "" {
			fetched, err := u.fetcher.FetchObject(ctx, createObject.Object, &account)
			if err != nil {
				return err
			}
			raw = fetched
		}
	}
	if announce, ok := extractOutboxAnnounce(raw); ok {
		return u.hydrateRemoteAnnounce(ctx, account, expectedActor, announce)
	}
	return u.hydrateRawObject(ctx, account, raw, expectedActor, false)
}

type outboxCreateObject struct {
	Actor     string
	Object    string
	ObjectRaw json.RawMessage
}

func extractOutboxCreateObject(raw []byte) (outboxCreateObject, bool) {
	var doc struct {
		Type   string          `json:"type"`
		Actor  json.RawMessage `json:"actor"`
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Type != "Create" || len(doc.Object) == 0 {
		return outboxCreateObject{}, false
	}
	actor, _, _ := ExtractIDAndInbox(doc.Actor)
	object, _, err := ExtractIDAndInbox(doc.Object)
	if err == nil && object != "" {
		return outboxCreateObject{Actor: actor, Object: object}, true
	}
	var embedded map[string]any
	if err := json.Unmarshal(doc.Object, &embedded); err != nil {
		return outboxCreateObject{}, false
	}
	return outboxCreateObject{Actor: actor, ObjectRaw: doc.Object}, true
}

type outboxAnnounce struct {
	ID        string
	Actor     string
	Object    string
	ObjectRaw json.RawMessage
	Published time.Time
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
	published, err := time.Parse(time.RFC3339, doc.Published)
	if err != nil {
		published = time.Now().UTC()
	}
	return outboxAnnounce{ID: doc.ID, Actor: actor, Object: object, ObjectRaw: doc.Object, Published: published}, true
}

func (u HydrateRemoteObjectUseCase) hydrateRemoteAnnounce(ctx context.Context, account models.Account, expectedActor string, announce outboxAnnounce) error {
	if u.boosts == nil || announce.Object == "" || announce.Actor == "" {
		return nil
	}
	if expectedActor != "" && announce.Actor != expectedActor {
		return nil
	}
	if _, err := u.notes.GetNoteByURI(ctx, nil, announce.Object); err != nil {
		var embedded map[string]any
		if err := json.Unmarshal(announce.ObjectRaw, &embedded); err == nil {
			if raw, err := json.Marshal(embedded); err == nil {
				_ = u.hydrateRawObject(ctx, account, raw, "", false)
			}
		} else if fetched, err := u.fetcher.FetchObject(ctx, announce.Object, &account); err == nil {
			_ = u.hydrateRawObject(ctx, account, fetched, "", false)
		}
	}
	persist := func(ctx context.Context, tx *db.Tx) error {
		note, err := u.notes.GetNoteByURI(ctx, tx, announce.Object)
		if err != nil {
			return nil
		}
		uri := announce.ID
		if uri == "" {
			uri = announce.Object + "#announce-" + announce.Actor
		}
		_, err = u.boosts.CreateBoost(ctx, tx, repos.CreateBoostInput{LocalAccountID: account.ID, Actor: announce.Actor, NoteID: note.ID, URI: uri, PublishedAt: announce.Published})
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
