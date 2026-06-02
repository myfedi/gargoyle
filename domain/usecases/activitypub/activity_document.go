package activitypub

import (
	"bytes"
	"encoding/json"
	"fmt"
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
func NormalizeOutboxActivity(raw []byte, account models.Account, activityID string, objectID string, sanitizer ports.ContentSanitizer) ([]byte, *domainerrors.DomainError) {
	if activityID == "" || objectID == "" {
		return nil, domainerrors.New(domainerrors.ErrInternal, "activity and object ids are required")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	typeValue, _ := doc["type"].(string)
	if typeValue == "" {
		if _, ok := doc["content"]; ok {
			typeValue = "Note"
			doc["type"] = typeValue
		} else {
			return nil, domainerrors.New(domainerrors.ErrBadRequest, "activity type is required")
		}
	}
	if typeValue != "Create" {
		object := doc
		SanitizeObjectContent(object, sanitizer)
		if _, ok := object["@context"]; !ok {
			object["@context"] = "https://www.w3.org/ns/activitystreams"
		}
		if _, ok := object["id"]; !ok {
			object["id"] = account.URI + "/objects/" + objectID
		}
		if _, ok := object["attributedTo"]; !ok {
			object["attributedTo"] = account.URI
		}
		if _, ok := object["published"]; !ok {
			object["published"] = now
		}
		if _, ok := object["to"]; !ok {
			object["to"] = []string{"https://www.w3.org/ns/activitystreams#Public"}
		}
		if _, ok := object["cc"]; !ok {
			object["cc"] = []string{account.FollowersURI}
		}
		doc = map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": account.URI + "/activities/" + activityID, "type": "Create", "actor": account.URI, "published": now, "to": object["to"], "cc": object["cc"], "object": object}
	} else {
		if object, ok := doc["object"].(map[string]any); ok {
			SanitizeObjectContent(object, sanitizer)
		}
		if _, ok := doc["@context"]; !ok {
			doc["@context"] = "https://www.w3.org/ns/activitystreams"
		}
		if _, ok := doc["id"]; !ok {
			doc["id"] = account.URI + "/activities/" + activityID
		}
		doc["actor"] = account.URI
		if _, ok := doc["published"]; !ok {
			doc["published"] = now
		}
		if _, ok := doc["to"]; !ok {
			doc["to"] = []string{"https://www.w3.org/ns/activitystreams#Public"}
		}
		if _, ok := doc["cc"]; !ok {
			doc["cc"] = []string{account.FollowersURI}
		}
		if object, ok := doc["object"].(map[string]any); ok {
			if _, ok := object["to"]; !ok {
				object["to"] = doc["to"]
			}
			if _, ok := object["cc"]; !ok {
				object["cc"] = doc["cc"]
			}
		}
	}
	res, err := json.Marshal(doc)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return res, nil
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
	URI          string
	Content      string
	Visibility   string
	Sensitive    bool
	SpoilerText  string
	AttributedTo string
	InReplyToURI *string
	MentionURIs  []string
	To           []string
	CC           []string
	PublishedAt  time.Time
}

type extractedNoteJSON struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Content      string          `json:"content"`
	Summary      string          `json:"summary"`
	Visibility   string          `json:"visibility"`
	Sensitive    bool            `json:"sensitive"`
	AttributedTo string          `json:"attributedTo"`
	InReplyTo    *string         `json:"inReplyTo"`
	To           json.RawMessage `json:"to"`
	CC           json.RawMessage `json:"cc"`
	Tag          json.RawMessage `json:"tag"`
	Published    string          `json:"published"`
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
	if err := json.Unmarshal(activity.Object, &note); err != nil || note.Type != "Note" || note.ID == "" {
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
	if err := json.Unmarshal(activity.Object, &note); err != nil || note.Type != "Note" || note.ID == "" {
		return ExtractedNote{}, false
	}
	return extractedNoteFromJSON(note), true
}

func extractedNoteFromJSON(note extractedNoteJSON) ExtractedNote {
	publishedAt, err := time.Parse(time.RFC3339, note.Published)
	if err != nil {
		publishedAt = time.Now().UTC()
	}
	return ExtractedNote{URI: note.ID, Content: note.Content, Visibility: normalizedExtractedVisibility(note.Visibility), Sensitive: note.Sensitive, SpoilerText: note.Summary, AttributedTo: note.AttributedTo, InReplyToURI: note.InReplyTo, MentionURIs: extractMentionURIs(note.Tag), To: extractStringList(note.To), CC: extractStringList(note.CC), PublishedAt: publishedAt}
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
	accept := map[string]any{"@context": "https://www.w3.org/ns/activitystreams", "id": account.URI + "/accepts/" + follow.ID, "type": "Accept", "actor": account.URI, "object": json.RawMessage(followRaw)}
	return json.Marshal(accept)
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
		PublicKey         struct {
			PublicKeyPem string `json:"publicKeyPem"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(activity.Object, &actor); err != nil || actor.ID == "" || actor.PreferredUsername == "" || !isActorType(actor.Type) {
		return ExtractedActor{}, false
	}
	return ExtractedActor{URI: actor.ID, Type: actor.Type, Username: actor.PreferredUsername, Name: actor.Name, Summary: actor.Summary, URL: extractURLValue(actor.URL), AvatarURL: extractURLValue(actor.Icon), HeaderURL: extractURLValue(actor.Image), Inbox: actor.Inbox, Outbox: actor.Outbox, Followers: actor.Followers, Following: actor.Following, PublicKey: actor.PublicKey.PublicKeyPem}, true
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
	to := []string{"https://www.w3.org/ns/activitystreams#Public"}
	cc := []string{account.FollowersURI}
	if note.Visibility == "private" {
		to = []string{account.FollowersURI}
		cc = []string{}
	} else if note.Visibility == "direct" {
		to = []string{}
		cc = []string{}
	}
	published := note.PublishedAt
	if published.IsZero() {
		published = note.CreatedAt
	}
	object := map[string]any{
		"id":           note.URI,
		"type":         "Note",
		"attributedTo": account.URI,
		"content":      note.Content,
		"published":    published.UTC().Format(time.RFC3339),
		"to":           to,
		"cc":           cc,
	}
	if note.SpoilerText != "" {
		object["summary"] = note.SpoilerText
	}
	if note.Sensitive {
		object["sensitive"] = true
	}
	if note.InReplyToURI != nil && *note.InReplyToURI != "" {
		object["inReplyTo"] = *note.InReplyToURI
	}
	return json.Marshal(object)
}
