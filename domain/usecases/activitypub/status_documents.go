package activitypub

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

// StatusDocumentInput contains AP document fields for a local Note-like object.
type StatusDocumentInput struct {
	Account       models.Account
	Content       string
	ObjectType    string
	Visibility    string
	Sensitive     bool
	SpoilerText   string
	InReplyToURI  string
	Media         []models.MediaAttachment
	Mentions      []models.Account
	PollOptions   []string
	PollMultiple  bool
	PollExpiresAt *time.Time
	Host          string
}

func BuildStatusObject(input StatusDocumentInput) map[string]any {
	noteDoc := map[string]any{"type": NormalizeStatusObjectType(input.ObjectType), "content": input.Content, "visibility": input.Visibility, "sensitive": input.Sensitive}
	ApplyVisibilityAddressing(noteDoc, input.Visibility, &input.Account, input.Mentions)
	ApplyContentTags(noteDoc, input.Content)
	ApplyPollQuestion(noteDoc, input.ObjectType, input.PollOptions, input.PollMultiple, input.PollExpiresAt)
	ApplyMediaAttachments(noteDoc, input.Host, input.Media)
	if input.SpoilerText != "" {
		noteDoc["summary"] = input.SpoilerText
	}
	if input.InReplyToURI != "" {
		noteDoc["inReplyTo"] = input.InReplyToURI
	}
	return noteDoc
}

func BuildStatusObjectJSON(input StatusDocumentInput) ([]byte, *domainerrors.DomainError) {
	raw, err := json.Marshal(BuildStatusObject(input))
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return raw, nil
}

func BuildStatusUpdateActivity(account models.Account, note models.Note, media []models.MediaAttachment, mentions []models.Account, pollOptions []string, host, activityID string) ([]byte, *domainerrors.DomainError) {
	if activityID == "" {
		return nil, domainerrors.New(domainerrors.ErrInternal, "activity id is required")
	}
	noteDoc := StatusUpdateObject(account, note)
	ApplyVisibilityAddressing(noteDoc, note.Visibility, &account, mentions)
	ApplyContentTags(noteDoc, note.PlainText)
	ApplyPollOptions(noteDoc, note, pollOptions)
	ApplyMediaAttachments(noteDoc, host, media)
	raw, err := json.Marshal(map[string]any{activityStreamsContextKey: activityStreamsContextURI, "id": account.URI + "/updates/" + activityID, "type": "Update", "actor": account.URI, "to": noteDoc["to"], "cc": noteDoc["cc"], "object": noteDoc})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return raw, nil
}

func StatusUpdateObject(account models.Account, note models.Note) map[string]any {
	doc := map[string]any{"id": note.URI, "type": NormalizeStatusObjectType(note.ObjectType), "content": note.Content, "attributedTo": account.URI, "visibility": note.Visibility, "sensitive": note.Sensitive}
	if !note.PublishedAt.IsZero() {
		doc["published"] = note.PublishedAt.UTC().Format(time.RFC3339)
	}
	if note.EditedAt != nil && !note.EditedAt.IsZero() {
		doc["updated"] = note.EditedAt.UTC().Format(time.RFC3339)
	}
	if note.SpoilerText != "" {
		doc["summary"] = note.SpoilerText
	}
	if note.InReplyToURI != nil && *note.InReplyToURI != "" {
		doc["inReplyTo"] = *note.InReplyToURI
	}
	return doc
}

func ApplyPollQuestion(noteDoc map[string]any, objectType string, options []string, multiple bool, expiresAt *time.Time) {
	if NormalizeStatusObjectType(objectType) != "Question" {
		return
	}
	items := pollOptionItems(options)
	if multiple {
		noteDoc["anyOf"] = items
	} else {
		noteDoc["oneOf"] = items
	}
	if expiresAt != nil {
		noteDoc["endTime"] = expiresAt.UTC().Format(time.RFC3339)
	}
}

func ApplyPollOptions(noteDoc map[string]any, note models.Note, options []string) {
	if NormalizeStatusObjectType(note.ObjectType) != "Question" || len(options) == 0 {
		return
	}
	items := pollOptionItems(options)
	if note.PollMultiple {
		noteDoc["anyOf"] = items
	} else {
		noteDoc["oneOf"] = items
	}
	if note.PollExpiresAt != nil {
		noteDoc["endTime"] = note.PollExpiresAt.UTC().Format(time.RFC3339)
	}
}

func pollOptionItems(options []string) []map[string]string {
	items := make([]map[string]string, 0, len(options))
	for _, option := range options {
		items = append(items, map[string]string{"type": "Note", "name": option})
	}
	return items
}

func NormalizeStatusObjectType(objectType string) string {
	switch objectType {
	case "Article", "Page", "Question":
		return objectType
	default:
		return "Note"
	}
}

func ApplyMediaAttachments(noteDoc map[string]any, host string, media []models.MediaAttachment) {
	if len(media) == 0 {
		return
	}
	attachments := make([]map[string]string, 0, len(media))
	base := strings.TrimRight(host, "/")
	for _, item := range media {
		attachment := map[string]string{"type": "Document", "mediaType": item.ContentType, "url": base + "/media/" + item.ID + "/" + url.PathEscape(item.FileName)}
		if item.Description != "" {
			attachment["name"] = item.Description
		}
		attachments = append(attachments, attachment)
	}
	noteDoc["attachment"] = attachments
}

var documentHashtagPattern = regexp.MustCompile(`(^|\s)#([\p{L}\p{N}_]{1,64})`)
var documentCustomEmojiPattern = regexp.MustCompile(`:([A-Za-z0-9_+-]{2,64}):`)

func ApplyContentTags(noteDoc map[string]any, content string) {
	existing, _ := noteDoc["tag"].([]map[string]string)
	tags := append([]map[string]string{}, existing...)
	seen := map[string]bool{}
	for _, tag := range tags {
		seen[tag["type"]+":"+strings.ToLower(tag["name"])] = true
	}
	for _, match := range documentHashtagPattern.FindAllStringSubmatch(content, -1) {
		name := match[2]
		key := "Hashtag:" + strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		tags = append(tags, map[string]string{"type": "Hashtag", "name": "#" + name})
	}
	_ = documentCustomEmojiPattern
	if len(tags) > 0 {
		noteDoc["tag"] = tags
	}
}

func ApplyVisibilityAddressing(noteDoc map[string]any, visibility string, account *models.Account, mentions []models.Account) {
	public := activityStreamsPublicURI
	mentionURIs := make([]string, 0, len(mentions))
	tags := make([]map[string]string, 0, len(mentions))
	for _, mention := range mentions {
		mentionURIs = append(mentionURIs, mention.URI)
		acct := mention.Username
		if mention.Domain != nil && *mention.Domain != "" {
			acct += "@" + *mention.Domain
		}
		tags = append(tags, map[string]string{"type": "Mention", "href": mention.URI, "name": "@" + acct})
	}
	if len(tags) > 0 {
		noteDoc["tag"] = tags
	}
	switch visibility {
	case "direct":
		noteDoc["to"] = mentionURIs
		noteDoc["cc"] = []string{}
	case "private":
		noteDoc["to"] = append([]string{account.FollowersURI}, mentionURIs...)
		noteDoc["cc"] = []string{}
	case "unlisted":
		noteDoc["to"] = []string{public}
		noteDoc["cc"] = append([]string{account.FollowersURI}, mentionURIs...)
	default:
		noteDoc["to"] = append([]string{public}, mentionURIs...)
		noteDoc["cc"] = []string{account.FollowersURI}
	}
}
