package activitypub

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// HydrateRemoteObjectUseCase fetches and persists missing remote ActivityPub
// objects, primarily reply parents discovered through inReplyTo links.
type HydrateRemoteObjectUseCase struct {
	fetcher            ports.RemoteObjectFetcher
	txProvider         db.TxProvider
	activities         repos.ActivitiesRepository
	notes              repos.NotesRepository
	media              repos.MediaRepository
	mediaStorage       ports.MediaStorage
	remoteMediaFetcher ports.RemoteMediaFetcher
	remotes            repos.RemoteAccountsRepository
	sanitizer          ports.ContentSanitizer
}

type HydrateRemoteObjectConfig struct {
	Fetcher            ports.RemoteObjectFetcher
	TxProvider         db.TxProvider
	ActivitiesRepo     repos.ActivitiesRepository
	NotesRepo          repos.NotesRepository
	MediaRepo          repos.MediaRepository
	MediaStorage       ports.MediaStorage
	RemoteMediaFetcher ports.RemoteMediaFetcher
	RemoteAccountsRepo repos.RemoteAccountsRepository
	Sanitizer          ports.ContentSanitizer
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
	return HydrateRemoteObjectUseCase{fetcher: cfg.Fetcher, txProvider: cfg.TxProvider, activities: cfg.ActivitiesRepo, notes: cfg.NotesRepo, media: cfg.MediaRepo, mediaStorage: cfg.MediaStorage, remoteMediaFetcher: cfg.RemoteMediaFetcher, remotes: cfg.RemoteAccountsRepo, sanitizer: cfg.Sanitizer}
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

func (u HydrateRemoteObjectUseCase) HydrateRemoteReplies(ctx context.Context, account models.Account, objectURI string) error {
	raw, err := u.fetcher.FetchObject(ctx, objectURI, &account)
	if err != nil {
		return err
	}
	return u.hydrateReplyCollection(ctx, account, objectURI, raw, 0, map[string]bool{objectURI: true})
}

func (u HydrateRemoteObjectUseCase) hydrateReplyCollection(ctx context.Context, account models.Account, parentURI string, raw []byte, depth int, seen map[string]bool) error {
	if depth >= 4 {
		return nil
	}
	items, links := extractReplyCollection(raw)
	u.hydrateReplyItems(ctx, account, parentURI, items)
	pages := u.fetchReplyPages(ctx, account, links, seen)
	for _, page := range pages {
		if err := u.hydrateReplyCollection(ctx, account, parentURI, page, depth+1, seen); err != nil {
			return err
		}
	}
	return nil
}

func (u HydrateRemoteObjectUseCase) hydrateReplyItems(ctx context.Context, account models.Account, parentURI string, items []json.RawMessage) {
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, item := range items {
		item := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			_ = u.hydrateReplyItem(ctx, account, parentURI, item)
		}()
	}
	wg.Wait()
}

func (u HydrateRemoteObjectUseCase) fetchReplyPages(ctx context.Context, account models.Account, links []string, seen map[string]bool) [][]byte {
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	var mu sync.Mutex
	pages := make([][]byte, 0, len(links))
	for _, link := range links {
		if link == "" || seen[link] {
			continue
		}
		seen[link] = true
		link := link
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			page, err := u.fetcher.FetchObject(ctx, link, &account)
			if err != nil {
				return
			}
			mu.Lock()
			pages = append(pages, page)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return pages
}

func (u HydrateRemoteObjectUseCase) hydrateReplyItem(ctx context.Context, account models.Account, parentURI string, item json.RawMessage) error {
	if fetchedNoteRepliesTo(item, parentURI) {
		return u.HydrateRawObject(ctx, account, item, "")
	}
	uri := replyItemURI(item)
	if uri == "" {
		return nil
	}
	raw, err := u.fetcher.FetchObject(ctx, uri, &account)
	if err != nil {
		return err
	}
	if !fetchedNoteRepliesTo(raw, parentURI) {
		return nil
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
	author := u.fetchRemoteAuthor(ctx, account, note.AttributedTo)
	persist := func(ctx context.Context, tx *db.Tx) error {
		if existing, err := u.notes.GetNoteByURI(ctx, tx, note.URI); err == nil {
			if err := u.ensureFetchedNoteAuthor(ctx, tx, author); err != nil {
				return err
			}
			return u.ensureFetchedNoteMedia(ctx, tx, account, existing.ID, note.Media)
		} else if err != sql.ErrNoRows {
			return err
		}
		if err := u.ensureFetchedNoteAuthor(ctx, tx, author); err != nil {
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
		created, err := u.notes.CreateNote(ctx, tx, repos.CreateNoteInput{LocalAccountID: account.ID, ActivityID: activity.ID, URI: note.URI, Content: u.sanitizer.SanitizeHTML(note.Content), PlainText: u.sanitizer.StripHTMLFromText(note.Content), ObjectType: note.Type, PollMultiple: note.PollMultiple, PollExpiresAt: note.PollExpiresAt, Hashtags: note.Hashtags, Emojis: note.Emojis, Visibility: note.Visibility, Sensitive: note.Sensitive, SpoilerText: note.SpoilerText, AttributedTo: note.AttributedTo, InReplyToID: replyID, InReplyToURI: replyURI, PublishedAt: publishedAt})
		if err != nil {
			return err
		}
		return u.ensureFetchedNoteMedia(ctx, tx, account, created.ID, note.Media)
	}
	if u.txProvider == nil {
		return persist(ctx, nil)
	}
	return u.txProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		return persist(ctx, &tx)
	})
}

func (u HydrateRemoteObjectUseCase) fetchRemoteAuthor(ctx context.Context, signer models.Account, actorURI string) *models.Account {
	if u.remotes == nil || actorURI == "" {
		return nil
	}
	if existing, err := u.remotes.GetRemoteAccountByURI(ctx, nil, actorURI); err == nil && existing.AvatarURL != nil && *existing.AvatarURL != "" {
		return nil
	}
	raw, err := u.fetcher.FetchObject(ctx, actorURI, &signer)
	if err != nil {
		return nil
	}
	author, ok := extractRemoteActor(raw)
	if !ok || author.URI != actorURI {
		return nil
	}
	return &author
}

func (u HydrateRemoteObjectUseCase) ensureFetchedNoteAuthor(ctx context.Context, tx *db.Tx, author *models.Account) error {
	if u.remotes == nil || author == nil {
		return nil
	}
	if existing, err := u.remotes.GetRemoteAccountByURI(ctx, tx, author.URI); err == nil && existing.AvatarURL != nil && *existing.AvatarURL != "" {
		return nil
	}
	_, err := u.remotes.UpsertRemoteAccount(ctx, tx, *author)
	return err
}

func extractRemoteActor(raw []byte) (models.Account, bool) {
	var doc struct {
		ID                string          `json:"id"`
		Type              string          `json:"type"`
		PreferredUsername string          `json:"preferredUsername"`
		Name              string          `json:"name"`
		Summary           string          `json:"summary"`
		URL               json.RawMessage `json:"url"`
		Icon              json.RawMessage `json:"icon"`
		Image             json.RawMessage `json:"image"`
		Inbox             string          `json:"inbox"`
		Outbox            string          `json:"outbox"`
		Followers         string          `json:"followers"`
		Following         string          `json:"following"`
		Locked            bool            `json:"manuallyApprovesFollowers"`
		PublicKey         struct {
			PublicKeyPem string `json:"publicKeyPem"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.ID == "" || doc.PreferredUsername == "" {
		return models.Account{}, false
	}
	domain := ""
	if parsed, err := url.Parse(doc.ID); err == nil {
		domain = parsed.Host
	}
	actorURL := remoteActorURL(doc.URL)
	return models.Account{ID: remoteActorID(doc.ID), Username: doc.PreferredUsername, Domain: stringPtr(domain), DisplayName: stringPtr(firstNonEmpty(doc.Name, doc.PreferredUsername)), Summary: stringPtr(doc.Summary), URI: doc.ID, URL: stringPtr(firstNonEmpty(actorURL, doc.ID)), AvatarURL: stringPtr(remoteActorURL(doc.Icon)), HeaderURL: stringPtr(remoteActorURL(doc.Image)), InboxURI: doc.Inbox, OutboxURI: stringPtr(doc.Outbox), FollowingURI: doc.Following, FollowersURI: doc.Followers, PublicKey: doc.PublicKey.PublicKeyPem, ActorType: models.ActorTypePerson, Locked: doc.Locked}, true
}

func remoteActorID(actor string) string {
	return "remote:" + base64.RawURLEncoding.EncodeToString([]byte(actor))
}

func remoteActorURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	var object struct {
		URL  string `json:"url"`
		Href string `json:"href"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return ""
	}
	if object.URL != "" {
		return object.URL
	}
	return object.Href
}

func (u HydrateRemoteObjectUseCase) ensureFetchedNoteMedia(ctx context.Context, tx *db.Tx, account models.Account, noteID string, attachments []ExtractedMediaAttachment) error {
	if u.media == nil || len(attachments) == 0 {
		return nil
	}
	existing, err := u.media.ListMediaForNote(ctx, tx, noteID)
	if err != nil {
		return err
	}
	existingRemoteURLs := map[string]bool{}
	for _, media := range existing {
		if media.RemoteURL != nil && *media.RemoteURL != "" {
			existingRemoteURLs[*media.RemoteURL] = true
		}
	}
	now := time.Now().UTC()
	for _, attachment := range attachments {
		if attachment.URL == "" || !(strings.HasPrefix(attachment.URL, "https://") || strings.HasPrefix(attachment.URL, "http://")) {
			continue
		}
		if existingRemoteURLs[attachment.URL] {
			continue
		}
		if existing, err := u.media.GetMediaAttachmentByRemoteURL(ctx, tx, attachment.URL); err == nil {
			if err := u.media.AttachMediaToNote(ctx, tx, noteID, existing.ID); err != nil {
				return err
			}
			continue
		}
		input := repos.CreateMediaAttachmentInput{LocalAccountID: account.ID, FileName: remoteMediaFileName(attachment.URL), ContentType: remoteMediaContentType(attachment), RemoteURL: &attachment.URL, RemoteFetchedAt: &now, RemoteLastAccessedAt: &now, Description: attachment.Description}
		if u.mediaStorage != nil && u.remoteMediaFetcher != nil {
			fetched, err := u.remoteMediaFetcher.FetchMedia(ctx, attachment.URL, 10<<20)
			if err == nil {
				contentType, ok := cachedRemoteMediaContentType(fetched)
				if ok {
					cacheKey := remoteMediaCacheKey(attachment.URL)
					fileName := fetched.FileName
					if strings.TrimSpace(fileName) == "" || fileName == "." || fileName == "/" {
						fileName = input.FileName
					}
					storagePath, saveErr := u.mediaStorage.SaveMedia(ctx, cacheKey, fileName, fetched.Data)
					if saveErr == nil {
						input.FileName = fileName
						input.ContentType = contentType
						input.StoragePath = storagePath
						input.FileSize = int64(len(fetched.Data))
					}
				}
			}
		}
		media, err := u.media.CreateMediaAttachment(ctx, tx, input)
		if err != nil {
			return err
		}
		if err := u.media.AttachMediaToNote(ctx, tx, noteID, media.ID); err != nil {
			return err
		}
	}
	return nil
}

func remoteMediaFileName(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "remote-media"
	}
	name := filepath.Base(parsed.Path)
	if name == "." || name == "/" || name == "" {
		return "remote-media"
	}
	return name
}

func remoteMediaContentType(attachment ExtractedMediaAttachment) string {
	if attachment.MediaType != "" {
		return attachment.MediaType
	}
	ext := strings.ToLower(filepath.Ext(remoteMediaFileName(attachment.URL)))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg", ".oga":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}

func cachedRemoteMediaContentType(fetched ports.FetchedRemoteMedia) (string, bool) {
	declared := strings.ToLower(strings.TrimSpace(strings.Split(fetched.ContentType, ";")[0]))
	sniffed := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(fetched.Data), ";")[0]))
	allowed := map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true, "video/mp4": true, "audio/mpeg": true, "audio/ogg": true, "audio/wav": true}
	if allowed[sniffed] {
		return sniffed, true
	}
	if allowed[declared] {
		return declared, true
	}
	return "", false
}

func remoteMediaCacheKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "remote-" + hex.EncodeToString(sum[:])
}

func replyItemURI(raw json.RawMessage) string {
	var uri string
	if err := json.Unmarshal(raw, &uri); err == nil {
		return uri
	}
	var link struct {
		ID   string `json:"id"`
		Href string `json:"href"`
	}
	if err := json.Unmarshal(raw, &link); err != nil {
		return ""
	}
	if link.ID != "" {
		return link.ID
	}
	return link.Href
}

func extractReplyCollection(raw []byte) ([]json.RawMessage, []string) {
	var doc struct {
		Replies json.RawMessage `json:"replies"`
		Items   json.RawMessage `json:"items"`
		Ordered json.RawMessage `json:"orderedItems"`
		First   json.RawMessage `json:"first"`
		Next    json.RawMessage `json:"next"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, nil
	}
	items := append(rawItems(doc.Items), rawItems(doc.Ordered)...)
	links := []string{}
	for _, child := range []json.RawMessage{doc.Replies, doc.First, doc.Next} {
		childItems, childLinks := rawCollection(child)
		items = append(items, childItems...)
		links = append(links, childLinks...)
	}
	return items, uniqueStrings(links)
}

func rawItems(raw json.RawMessage) []json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		return items
	}
	return nil
}

func rawCollection(raw json.RawMessage) ([]json.RawMessage, []string) {
	if len(raw) == 0 {
		return nil, nil
	}
	var link string
	if err := json.Unmarshal(raw, &link); err == nil && link != "" {
		return nil, []string{link}
	}
	return extractReplyCollection(raw)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	res := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		res = append(res, value)
	}
	return res
}

func fetchedNoteRepliesTo(raw []byte, parentURI string) bool {
	note, ok := extractFetchedNote(raw)
	return ok && note.InReplyToURI != nil && *note.InReplyToURI == parentURI
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
