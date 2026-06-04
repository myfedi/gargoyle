package mastodon

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

func (u UseCase) cacheRemoteOutbox(ctx context.Context, localAccount *models.Account, remote models.Account) *domainerrors.DomainError {
	if remote.OutboxURI == nil || *remote.OutboxURI == "" || localAccount == nil {
		return nil
	}
	raw, err := u.cfg.RemoteObjectFetcher.FetchObject(ctx, *remote.OutboxURI, localAccount)
	if err != nil {
		return nil
	}
	items, next := outboxItems(raw)
	if len(items) == 0 && next != "" {
		raw, err = u.cfg.RemoteObjectFetcher.FetchObject(ctx, next, localAccount)
		if err != nil {
			return nil
		}
		items, _ = outboxItems(raw)
	}
	for _, item := range items {
		_ = u.cacheRemoteOutboxItem(ctx, localAccount, remote, item)
	}
	return nil
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

func (u UseCase) cacheRemoteOutboxItem(ctx context.Context, localAccount *models.Account, remote models.Account, raw json.RawMessage) error {
	var itemURL string
	if err := json.Unmarshal(raw, &itemURL); err == nil && itemURL != "" {
		fetched, err := u.cfg.RemoteObjectFetcher.FetchObject(ctx, itemURL, localAccount)
		if err != nil {
			return err
		}
		raw = fetched
	}
	note, ok := apUsecases.ExtractNote(raw)
	if !ok {
		note, ok = apUsecases.ExtractStandaloneNote(raw)
	}
	if !ok || note.URI == "" || note.AttributedTo != remote.URI || note.Visibility == "direct" {
		return nil
	}
	if _, err := u.cfg.NotesRepo.GetNoteByURI(ctx, nil, note.URI); err == nil {
		return nil
	} else if err != sql.ErrNoRows {
		return err
	}
	activityID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return err
	}
	activity, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, nil, repos.CreateActivityInput{LocalAccountID: localAccount.ID, Direction: models.ActivityDirectionInbox, Type: "Create", Actor: note.AttributedTo, Object: note.URI, RawJSON: string(raw)})
	if err != nil {
		return err
	}
	if activity.ID == "" {
		activity.ID = activityID
	}
	replyID, replyURI := u.remoteReplyIDs(ctx, localAccount, note)
	publishedAt := note.PublishedAt
	if publishedAt.IsZero() {
		publishedAt = time.Now().UTC()
	}
	_, err = u.cfg.NotesRepo.CreateNote(ctx, nil, repos.CreateNoteInput{LocalAccountID: localAccount.ID, ActivityID: activity.ID, URI: note.URI, Content: u.cfg.ContentSanitizer.SanitizeHTML(note.Content), PlainText: u.cfg.ContentSanitizer.StripHTMLFromText(note.Content), ObjectType: note.Type, PollMultiple: note.PollMultiple, PollExpiresAt: note.PollExpiresAt, Hashtags: note.Hashtags, Emojis: note.Emojis, Visibility: note.Visibility, Sensitive: note.Sensitive, SpoilerText: note.SpoilerText, AttributedTo: note.AttributedTo, InReplyToID: replyID, InReplyToURI: replyURI, PublishedAt: publishedAt})
	return err
}

func (u UseCase) remoteReplyIDs(ctx context.Context, localAccount *models.Account, note apUsecases.ExtractedNote) (*string, *string) {
	if note.InReplyToURI == nil || *note.InReplyToURI == "" {
		return nil, note.InReplyToURI
	}
	parent, err := u.cfg.NotesRepo.GetNoteByURI(ctx, nil, *note.InReplyToURI)
	if err != nil || parent.LocalAccountID != localAccount.ID {
		return nil, note.InReplyToURI
	}
	return &parent.ID, &parent.URI
}
