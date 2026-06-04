package activitypub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports"
)

// ParsedActivity is the normalized subset of an ActivityPub activity used by the application.
type ParsedActivity struct {
	Type   string
	Actor  string
	Object string
	Inbox  string
}

// ParseActivity extracts the activity type, actor, object, and optional inbox
// from a raw ActivityPub activity document.
func ParseActivity(raw []byte) (ParsedActivity, *domainerrors.DomainError) {
	var envelope struct {
		Context json.RawMessage `json:"@context,omitempty"`
		ID      string          `json:"id,omitempty"`
		Type    string          `json:"type"`
		Actor   json.RawMessage `json:"actor"`
		Object  json.RawMessage `json:"object,omitempty"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ParsedActivity{}, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	if envelope.Type == "" {
		return ParsedActivity{}, domainerrors.New(domainerrors.ErrBadRequest, "activity type is required")
	}
	actor, inbox, err := ExtractIDAndInbox(envelope.Actor)
	if err != nil {
		return ParsedActivity{}, domainerrors.NewErr(domainerrors.ErrBadRequest, fmt.Errorf("invalid actor: %w", err))
	}
	object, _, err := ExtractIDAndInbox(envelope.Object)
	if len(envelope.Object) > 0 && err != nil {
		return ParsedActivity{}, domainerrors.NewErr(domainerrors.ErrBadRequest, fmt.Errorf("invalid object: %w", err))
	}
	return ParsedActivity{Type: envelope.Type, Actor: actor, Object: object, Inbox: inbox}, nil
}

// NormalizeOutboxActivity turns a local outbox submission into a Create activity
// when needed, assigns local IDs, enforces local actor ownership, and sanitizes
// Note content before persistence and delivery.
func NormalizeOutboxActivity(raw []byte, account models.Account, activityID, objectID string, sanitizer ports.ContentSanitizer) ([]byte, *domainerrors.DomainError) {
	if activityID == "" || objectID == "" {
		return nil, domainerrors.New(domainerrors.ErrInternal, "activity and object ids are required")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}

	typeValue, derr := normalizeOutboxType(doc)
	if derr != nil {
		return nil, derr
	}
	now := time.Now().UTC().Format(time.RFC3339)

	// Local outbox clients may submit either a bare object or a full Create.
	// Normalize both into a Create while preserving caller-provided addressing.
	if typeValue == "Create" {
		normalizeCreateActivity(doc, account, activityID, now, sanitizer)
	} else {
		doc = wrapObjectInCreateActivity(doc, account, activityID, objectID, now, sanitizer)
	}
	res, err := json.Marshal(doc)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return res, nil
}

// normalizeOutboxType accepts the Mastodon-style shorthand where a bare object
// with content but no explicit type is treated as a Note.
func normalizeOutboxType(doc map[string]any) (string, *domainerrors.DomainError) {
	typeValue, _ := doc["type"].(string)
	if typeValue != "" {
		return typeValue, nil
	}
	if _, ok := doc["content"]; !ok {
		return "", domainerrors.New(domainerrors.ErrBadRequest, "activity type is required")
	}

	doc["type"] = "Note"
	return "Note", nil
}

// wrapObjectInCreateActivity converts a local bare object submission into the
// server-owned Create activity we persist and deliver.
func wrapObjectInCreateActivity(doc map[string]any, account models.Account, activityID, objectID, published string, sanitizer ports.ContentSanitizer) map[string]any {
	object := doc
	SanitizeObjectContent(object, sanitizer)
	ensureObjectDefaults(object, account, objectID, published)

	return map[string]any{
		activityStreamsContextKey: activityStreamsContextURI,
		"id":                      account.URI + activityPathSegment + activityID,
		"type":                    "Create",
		"actor":                   account.URI,
		"published":               published,
		"to":                      object["to"],
		"cc":                      object["cc"],
		"object":                  object,
	}
}

// normalizeCreateActivity enforces local actor ownership and fills defaults on
// already-wrapped Create activities without overwriting explicit addressing.
func normalizeCreateActivity(doc map[string]any, account models.Account, activityID, published string, sanitizer ports.ContentSanitizer) {
	if object, ok := doc["object"].(map[string]any); ok {
		SanitizeObjectContent(object, sanitizer)
	}

	ensureValue(doc, activityStreamsContextKey, activityStreamsContextURI)
	ensureValue(doc, "id", account.URI+activityPathSegment+activityID)
	doc["actor"] = account.URI
	ensureValue(doc, "published", published)
	ensureValue(doc, "to", []string{activityStreamsPublicURI})
	ensureValue(doc, "cc", []string{account.FollowersURI})

	if object, ok := doc["object"].(map[string]any); ok {
		ensureValue(object, "to", doc["to"])
		ensureValue(object, "cc", doc["cc"])
	}
}

// ensureObjectDefaults applies safe local defaults for objects the server wraps.
func ensureObjectDefaults(object map[string]any, account models.Account, objectID, published string) {
	ensureValue(object, activityStreamsContextKey, activityStreamsContextURI)
	ensureValue(object, "id", account.URI+"/objects/"+objectID)
	ensureValue(object, "attributedTo", account.URI)
	ensureValue(object, "published", published)
	ensureValue(object, "to", []string{activityStreamsPublicURI})
	ensureValue(object, "cc", []string{account.FollowersURI})
}

// ensureValue fills a missing ActivityPub field while preserving explicit input.
func ensureValue(doc map[string]any, key string, value any) {
	if _, ok := doc[key]; !ok {
		doc[key] = value
	}
}

// SanitizeObjectContent applies the configured sanitizer to Note-like objects.
func SanitizeObjectContent(object map[string]any, sanitizer ports.ContentSanitizer) {
	if typ, _ := object["type"].(string); typ != "" && typ != "Note" {
		return
	}
	if content, ok := object["content"].(string); ok {
		object["content"] = sanitizer.SanitizeHTML(content)
	}
}

// ExtractedNote is the domain data persisted when an ActivityPub Note is found.
type ExtractedNote struct {
	URI           string
	Type          string
	Content       string
	Visibility    string
	Sensitive     bool
	SpoilerText   string
	AttributedTo  string
	InReplyToURI  *string
	MentionURIs   []string
	To            []string
	CC            []string
	PublishedAt   time.Time
	PollOptions   []string
	PollMultiple  bool
	PollExpiresAt *time.Time
	Hashtags      []string
	Emojis        []models.CustomEmoji
}

type extractedNoteJSON struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Content      string          `json:"content"`
	Name         string          `json:"name"`
	Summary      string          `json:"summary"`
	Visibility   string          `json:"visibility"`
	Sensitive    bool            `json:"sensitive"`
	AttributedTo string          `json:"attributedTo"`
	InReplyTo    *string         `json:"inReplyTo"`
	To           json.RawMessage `json:"to"`
	CC           json.RawMessage `json:"cc"`
	Tag          json.RawMessage `json:"tag"`
	Published    string          `json:"published"`
	EndTime      string          `json:"endTime"`
	OneOf        json.RawMessage `json:"oneOf"`
	AnyOf        json.RawMessage `json:"anyOf"`
}

func normalizedExtractedVisibility(visibility string) string {
	switch visibility {
	case "public", "unlisted", "private", "direct":
		return visibility
	default:
		return "public"
	}
}

// ExtractNote returns the Note embedded in a Create activity, when present.
func ExtractNote(raw []byte) (ExtractedNote, bool) {
	var activity struct {
		Type   string          `json:"type"`
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(raw, &activity); err != nil || activity.Type != "Create" || len(activity.Object) == 0 {
		return ExtractedNote{}, false
	}
	var note extractedNoteJSON
	if err := json.Unmarshal(activity.Object, &note); err != nil || !isNoteLikeObjectType(note.Type) || note.ID == "" {
		return ExtractedNote{}, false
	}
	return extractedNoteFromJSON(note), true
}

// ExtractStandaloneNote returns a Note document fetched directly by URI.
func ExtractStandaloneNote(raw []byte) (ExtractedNote, bool) {
	var note extractedNoteJSON
	if err := json.Unmarshal(raw, &note); err != nil || !isNoteLikeObjectType(note.Type) || note.ID == "" {
		return ExtractedNote{}, false
	}
	return extractedNoteFromJSON(note), true
}

// ExtractNoteObject returns a Note from an activity object, used for Updates.
func ExtractNoteObject(raw []byte) (ExtractedNote, bool) {
	var activity struct {
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(raw, &activity); err != nil || len(activity.Object) == 0 {
		return ExtractedNote{}, false
	}
	var note extractedNoteJSON
	if err := json.Unmarshal(activity.Object, &note); err != nil || !isNoteLikeObjectType(note.Type) || note.ID == "" {
		return ExtractedNote{}, false
	}
	return extractedNoteFromJSON(note), true
}

func isNoteLikeObjectType(value string) bool {
	switch value {
	case "Note", "Article", "Page", "Question":
		return true
	default:
		return false
	}
}

func normalizedObjectType(value string) string {
	if isNoteLikeObjectType(value) {
		return value
	}
	return "Note"
}

func extractedNoteFromJSON(note extractedNoteJSON) ExtractedNote {
	publishedAt, err := time.Parse(time.RFC3339, note.Published)
	if err != nil {
		publishedAt = time.Now().UTC()
	}
	content := note.Content
	if content == "" {
		content = note.Name
	}
	var expiresAt *time.Time
	if parsed, err := time.Parse(time.RFC3339, note.EndTime); err == nil {
		expiresAt = &parsed
	}
	pollOptions, multiple := extractPollOptions(note.OneOf, note.AnyOf)
	return ExtractedNote{URI: note.ID, Type: normalizedObjectType(note.Type), Content: content, Visibility: normalizedExtractedVisibility(note.Visibility), Sensitive: note.Sensitive, SpoilerText: note.Summary, AttributedTo: note.AttributedTo, InReplyToURI: note.InReplyTo, MentionURIs: extractMentionURIs(note.Tag), To: extractStringList(note.To), CC: extractStringList(note.CC), PublishedAt: publishedAt, PollOptions: pollOptions, PollMultiple: multiple, PollExpiresAt: expiresAt, Hashtags: extractHashtags(note.Tag), Emojis: extractEmojis(note.Tag)}
}

// ExtractLocalRecipientUsernames returns local actor usernames referenced by an
// ActivityPub activity. Shared inbox delivery does not carry the target username
// in the route, so this conservative extractor looks at common addressing and
// ownership fields and maps local actor URIs back to usernames.
func ExtractLocalRecipientUsernames(raw []byte, host string) []string {
	base := strings.TrimRight(host, "/") + "/users/"
	if host == "" {
		return nil
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	seen := map[string]bool{}
	collectLocalUsernames(doc, base, seen)
	res := make([]string, 0, len(seen))
	for username := range seen {
		res = append(res, username)
	}
	return res
}

func collectLocalUsernames(value any, base string, seen map[string]bool) {
	switch v := value.(type) {
	case string:
		if username := localUsernameFromURI(v, base); username != "" {
			seen[username] = true
		}
	case []any:
		for _, item := range v {
			collectLocalUsernames(item, base, seen)
		}
	case map[string]any:
		for _, key := range []string{"actor", "object", "target", "attributedTo", "to", "cc", "bto", "bcc", "audience", "href", "tag"} {
			if child, ok := v[key]; ok {
				collectLocalUsernames(child, base, seen)
			}
		}
	}
}

func localUsernameFromURI(raw, base string) string {
	if !strings.HasPrefix(raw, base) {
		return ""
	}
	rest := strings.TrimPrefix(raw, base)
	segment, _, _ := strings.Cut(rest, "/")
	username, err := url.PathUnescape(segment)
	if err != nil || username == "" {
		return ""
	}
	return username
}

func extractPollOptions(oneOf, anyOf json.RawMessage) ([]string, bool) {
	raw := oneOf
	multiple := false
	if len(anyOf) > 0 {
		raw = anyOf
		multiple = true
	}
	if len(raw) == 0 {
		return nil, false
	}
	var rows []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, false
	}
	options := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Name) != "" {
			options = append(options, strings.TrimSpace(row.Name))
		}
	}
	return options, multiple
}

func extractStringList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil && single != "" {
		return []string{single}
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	return nil
}

func extractHashtags(raw json.RawMessage) []string {
	return extractTagNames(raw, "Hashtag")
}

func extractTagNames(raw json.RawMessage, typ string) []string {
	if len(raw) == 0 {
		return nil
	}
	var tags []struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &tags); err != nil {
		var tag struct {
			Type string `json:"type"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &tag); err != nil {
			return nil
		}
		tags = []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		}{tag}
	}
	seen := map[string]bool{}
	res := make([]string, 0, len(tags))
	for _, tag := range tags {
		name := strings.TrimPrefix(strings.TrimSpace(tag.Name), "#")
		if tag.Type == typ && name != "" && !seen[strings.ToLower(name)] {
			seen[strings.ToLower(name)] = true
			res = append(res, name)
		}
	}
	return res
}

func extractEmojis(raw json.RawMessage) []models.CustomEmoji {
	if len(raw) == 0 {
		return nil
	}
	var tags []struct {
		Type string `json:"type"`
		Name string `json:"name"`
		Icon struct {
			URL string `json:"url"`
		} `json:"icon"`
	}
	if err := json.Unmarshal(raw, &tags); err != nil {
		var tag struct {
			Type string `json:"type"`
			Name string `json:"name"`
			Icon struct {
				URL string `json:"url"`
			} `json:"icon"`
		}
		if err := json.Unmarshal(raw, &tag); err != nil {
			return nil
		}
		tags = []struct {
			Type string `json:"type"`
			Name string `json:"name"`
			Icon struct {
				URL string `json:"url"`
			} `json:"icon"`
		}{tag}
	}
	seen := map[string]bool{}
	emojis := make([]models.CustomEmoji, 0, len(tags))
	for _, tag := range tags {
		if tag.Type != "Emoji" || tag.Name == "" || tag.Icon.URL == "" {
			continue
		}
		shortcode := strings.Trim(tag.Name, ":")
		if shortcode == "" || seen[shortcode] {
			continue
		}
		seen[shortcode] = true
		emojis = append(emojis, models.CustomEmoji{Shortcode: shortcode, URL: tag.Icon.URL, StaticURL: tag.Icon.URL})
	}
	return emojis
}

func extractMentionURIs(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var tags []struct {
		Type string `json:"type"`
		Href string `json:"href"`
	}
	if err := json.Unmarshal(raw, &tags); err != nil {
		var tag struct {
			Type string `json:"type"`
			Href string `json:"href"`
		}
		if err := json.Unmarshal(raw, &tag); err != nil {
			return nil
		}
		tags = []struct {
			Type string `json:"type"`
			Href string `json:"href"`
		}{tag}
	}
	mentions := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag.Type == "Mention" && tag.Href != "" {
			mentions = append(mentions, tag.Href)
		}
	}
	return mentions
}

// ExtractedFollowObject is the normalized Follow object embedded in Accept/Reject activities.
type ExtractedFollowObject struct {
	Actor  string
	Object string
}

// ExtractFollowObject returns the Follow object embedded in an Accept or Reject activity.
func ExtractFollowObject(raw []byte) (ExtractedFollowObject, bool, error) {
	var activity struct {
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(raw, &activity); err != nil || len(activity.Object) == 0 {
		return ExtractedFollowObject{}, false, err
	}
	var follow struct {
		Type   string          `json:"type"`
		Actor  json.RawMessage `json:"actor"`
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(activity.Object, &follow); err != nil || follow.Type != "Follow" {
		return ExtractedFollowObject{}, false, err
	}
	actor, _, err := ExtractIDAndInbox(follow.Actor)
	if err != nil {
		return ExtractedFollowObject{}, false, err
	}
	object, _, err := ExtractIDAndInbox(follow.Object)
	if err != nil {
		return ExtractedFollowObject{}, false, err
	}
	return ExtractedFollowObject{Actor: actor, Object: object}, true, nil
}

// ExtractedUndoActivity is the embedded activity object inside an Undo.
type ExtractedUndoActivity struct {
	Type   string
	Actor  string
	Object string
}

// ExtractUndoActivity resolves an embedded Like/Announce/Follow activity inside
// an Undo. When peers send only the undone activity ID, the returned Object is
// that ID and Type is empty; callers may look up the original stored activity.
func ExtractUndoActivity(raw []byte) (ExtractedUndoActivity, error) {
	var doc struct {
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ExtractedUndoActivity{}, err
	}
	var objectID string
	if err := json.Unmarshal(doc.Object, &objectID); err == nil && objectID != "" {
		return ExtractedUndoActivity{Object: objectID}, nil
	}
	var obj struct {
		Type   string          `json:"type"`
		Actor  json.RawMessage `json:"actor"`
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(doc.Object, &obj); err != nil {
		return ExtractedUndoActivity{}, err
	}
	actor, _, err := ExtractIDAndInbox(obj.Actor)
	if err != nil {
		return ExtractedUndoActivity{}, err
	}
	object, _, err := ExtractIDAndInbox(obj.Object)
	if err != nil {
		return ExtractedUndoActivity{}, err
	}
	return ExtractedUndoActivity{Type: obj.Type, Actor: actor, Object: object}, nil
}

// ExtractUndoFollowActor resolves the actor whose Follow is being undone.
func ExtractUndoFollowActor(raw []byte) (string, error) {
	var doc struct {
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", err
	}
	var obj struct {
		Type  string          `json:"type"`
		Actor json.RawMessage `json:"actor"`
	}
	if err := json.Unmarshal(doc.Object, &obj); err != nil {
		actor, _, actorErr := ExtractIDAndInbox(doc.Object)
		if actorErr != nil {
			return "", err
		}
		return actor, nil
	}
	if obj.Type != "Follow" {
		return "", nil
	}
	actor, _, err := ExtractIDAndInbox(obj.Actor)
	return actor, err
}

func ExtractObjectIDByType(raw []byte, wantedType string) string {
	var activity struct {
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(raw, &activity); err != nil || len(activity.Object) == 0 {
		return ""
	}
	var object struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(activity.Object, &object); err != nil || object.Type != wantedType {
		return ""
	}
	return object.ID
}

func ExtractMoveTarget(raw []byte) string {
	var activity struct {
		Target json.RawMessage `json:"target"`
	}
	if err := json.Unmarshal(raw, &activity); err != nil || len(activity.Target) == 0 {
		return ""
	}
	target, _, _ := ExtractIDAndInbox(activity.Target)
	return target
}

// ExtractIDAndInbox accepts either a string ID or an object with id/inbox fields.
func ExtractIDAndInbox(raw json.RawMessage) (string, string, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, "", nil
	}
	var obj struct {
		ID    string `json:"id"`
		Inbox string `json:"inbox"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", "", err
	}
	return obj.ID, obj.Inbox, nil
}

// MarshalAccept creates the Accept activity sent in response to an inbound Follow.
func MarshalAccept(account models.Account, follow models.Follow, followRaw []byte) ([]byte, error) {
	accept := map[string]any{activityStreamsContextKey: activityStreamsContextURI, "id": account.URI + "/accepts/" + follow.ID, "type": "Accept", "actor": account.URI, "object": json.RawMessage(followRaw)}
	return json.Marshal(accept)
}

// MarshalReject creates the Reject activity sent when an inbound Follow is denied.
func MarshalReject(account models.Account, follow models.Follow, followRaw []byte) ([]byte, error) {
	reject := map[string]any{activityStreamsContextKey: activityStreamsContextURI, "id": account.URI + "/rejects/" + follow.ID, "type": "Reject", "actor": account.URI, "object": json.RawMessage(followRaw)}
	return json.Marshal(reject)
}

// ExtractedActor is the cacheable subset of an ActivityPub actor document embedded in an Update.
type ExtractedActor struct {
	URI       string
	Type      string
	Username  string
	Name      string
	Summary   string
	URL       string
	AvatarURL string
	HeaderURL string
	Inbox     string
	Outbox    string
	Followers string
	Following string
	PublicKey string
	Locked    bool
}

// ExtractActorObject returns an actor from an activity object, used for profile Updates.
func ExtractActorObject(raw []byte) (ExtractedActor, bool) {
	var activity struct {
		Object json.RawMessage `json:"object"`
	}
	if err := json.Unmarshal(raw, &activity); err != nil || len(activity.Object) == 0 {
		return ExtractedActor{}, false
	}
	var actor struct {
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
	if err := json.Unmarshal(activity.Object, &actor); err != nil || actor.ID == "" || actor.PreferredUsername == "" || !isActorType(actor.Type) {
		return ExtractedActor{}, false
	}
	return ExtractedActor{URI: actor.ID, Type: actor.Type, Username: actor.PreferredUsername, Name: actor.Name, Summary: actor.Summary, URL: extractURLValue(actor.URL), AvatarURL: extractURLValue(actor.Icon), HeaderURL: extractURLValue(actor.Image), Inbox: actor.Inbox, Outbox: actor.Outbox, Followers: actor.Followers, Following: actor.Following, PublicKey: actor.PublicKey.PublicKeyPem, Locked: actor.Locked}, true
}

func isActorType(value string) bool {
	switch value {
	case "Application", "Group", "Organization", "Person", "Service":
		return true
	default:
		return false
	}
}

func extractURLValue(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return single
	}
	var object struct {
		URL json.RawMessage `json:"url"`
	}
	if err := json.Unmarshal(raw, &object); err == nil && len(object.URL) > 0 {
		return extractURLValue(object.URL)
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return extractURLValue(list[0])
	}
	return ""
}

// MarshalFeaturedNoteObject serializes a stored local Note as an ActivityPub Note for featured collections.
func MarshalFeaturedNoteObject(note models.Note, account models.Account) ([]byte, error) {
	return json.Marshal(noteObjectDocument(note, account))
}
