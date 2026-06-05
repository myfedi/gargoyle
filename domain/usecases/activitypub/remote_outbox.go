package activitypub

import (
	"context"
	"encoding/json"

	"github.com/myfedi/gargoyle/domain/models"
)

// CacheRemoteOutbox fetches a remote actor outbox page and hydrates its Create/Note items.
func (u HydrateRemoteObjectUseCase) CacheRemoteOutbox(ctx context.Context, account models.Account, outboxURI, expectedActor string) error {
	if outboxURI == "" {
		return nil
	}
	raw, err := u.fetcher.FetchObject(ctx, outboxURI, &account)
	if err != nil {
		return err
	}
	items, next := outboxItems(raw)
	if len(items) == 0 && next != "" {
		raw, err = u.fetcher.FetchObject(ctx, next, &account)
		if err != nil {
			return err
		}
		items, _ = outboxItems(raw)
	}
	for _, item := range items {
		_ = u.cacheRemoteOutboxItem(ctx, account, expectedActor, item)
	}
	return nil
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
	return u.HydrateRawObject(ctx, account, raw, expectedActor)
}

func outboxItems(raw []byte) ([]json.RawMessage, string) {
	var doc struct {
		Type         string            `json:"type"`
		First        json.RawMessage   `json:"first"`
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
	return items, collectionRef(doc.First)
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
