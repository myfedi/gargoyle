package mastodon

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

// CreateStatus creates a local Note through the ActivityPub outbox workflow so
// Mastodon API posting and federation posting share the same normalization,
// persistence, and fan-out semantics.
func (u UseCase) CreateStatus(ctx context.Context, account *models.Account, input CreateStatusInput) (*CreateStatusResult, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	if strings.TrimSpace(input.Content) == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "status is required")
	}
	activityID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	objectID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	visibility := normalizedVisibility(input.Visibility)
	mentions, derr := u.resolveMentions(ctx, account, input.Content)
	if derr != nil {
		return nil, derr
	}
	if visibility == "direct" && len(mentions) == 0 {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "direct statuses require at least one mentioned recipient")
	}
	media, derr := u.statusMedia(ctx, account, input.MediaIDs)
	if derr != nil {
		return nil, derr
	}
	noteDoc := map[string]any{"type": "Note", "content": input.Content, "visibility": visibility, "sensitive": input.Sensitive}
	applyVisibilityAddressing(noteDoc, visibility, account, mentions)
	applyMediaAttachments(noteDoc, u.cfg.Host, media)
	if input.SpoilerText != "" {
		noteDoc["summary"] = input.SpoilerText
	}
	if input.InReplyToID != "" {
		parent, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, input.InReplyToID)
		if err != nil || parent.LocalAccountID != account.ID {
			return nil, domainerrors.New(domainerrors.ErrBadRequest, "in_reply_to_id is invalid")
		}
		noteDoc["inReplyTo"] = parent.URI
	}
	raw, err := json.Marshal(noteDoc)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res, derr := u.cfg.CreateOutboxUC.CreateOutboxActivity(ctx, apUsecases.CreateOutboxActivityInput{Username: account.Username, RawJSON: raw, ActivityID: activityID, ObjectID: objectID})
	if derr != nil {
		return nil, derr
	}
	extracted, ok := apUsecases.ExtractNote(res.RawJSON)
	if !ok {
		return nil, domainerrors.New(domainerrors.ErrInternal, "created activity did not contain a note")
	}
	note, err := u.cfg.NotesRepo.GetNoteByURI(ctx, nil, extracted.URI)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	for _, item := range media {
		if err := u.cfg.MediaRepo.AttachMediaToNote(ctx, nil, note.ID, item.ID); err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
	}
	if derr := u.persistMentions(ctx, account.ID, note.ID, mentions); derr != nil {
		return nil, derr
	}
	storedMentions, err := u.cfg.MentionsRepo.ListMentionsForNote(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if derr := u.createLocalStatusNotifications(ctx, account, note.ID, mentions); derr != nil {
		return nil, derr
	}
	note.Visibility = visibility
	note.Sensitive = input.Sensitive
	note.SpoilerText = input.SpoilerText
	return &CreateStatusResult{Note: *note, Account: res.Account, Media: media, Mentions: storedMentions, RawJSON: res.RawJSON, FollowerInboxes: res.FollowerInboxes, MentionInboxes: mentionInboxes(mentions)}, nil
}

func (u UseCase) statusMedia(ctx context.Context, account *models.Account, mediaIDs []string) ([]models.MediaAttachment, *domainerrors.DomainError) {
	media := make([]models.MediaAttachment, 0, len(mediaIDs))
	seen := map[string]bool{}
	for _, mediaID := range mediaIDs {
		mediaID = strings.TrimSpace(mediaID)
		if mediaID == "" || seen[mediaID] {
			continue
		}
		seen[mediaID] = true
		item, err := u.cfg.MediaRepo.GetMediaAttachmentByID(ctx, nil, mediaID)
		if err != nil || item.LocalAccountID != account.ID {
			return nil, domainerrors.New(domainerrors.ErrBadRequest, "media_ids contains invalid media")
		}
		media = append(media, *item)
	}
	return media, nil
}

func applyMediaAttachments(noteDoc map[string]any, host string, media []models.MediaAttachment) {
	if len(media) == 0 {
		return
	}
	attachments := make([]map[string]string, 0, len(media))
	base := strings.TrimRight(host, "/")
	for _, item := range media {
		attachment := map[string]string{"type": "Document", "mediaType": item.ContentType, "url": base + "/media/" + item.ID}
		if item.Description != "" {
			attachment["name"] = item.Description
		}
		attachments = append(attachments, attachment)
	}
	noteDoc["attachment"] = attachments
}

var remoteMentionPattern = regexp.MustCompile(`@([A-Za-z0-9_]+)@([A-Za-z0-9.-]+)`)

func (u UseCase) resolveMentions(ctx context.Context, account *models.Account, content string) ([]models.Account, *domainerrors.DomainError) {
	matches := remoteMentionPattern.FindAllStringSubmatch(content, -1)
	mentions := make([]models.Account, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		username := match[1]
		domain := strings.ToLower(match[2])
		acct := username + "@" + domain
		if seen[acct] {
			continue
		}
		seen[acct] = true
		if domain == strings.ToLower(u.cfg.Domain) {
			local, err := u.cfg.AccountsRepo.GetLocalAccountByUsername(ctx, nil, username)
			if err == nil {
				mentions = append(mentions, *local)
			}
			continue
		}
		remote, err := u.resolveAndCacheRemoteAccount(ctx, acct, account)
		if err == nil {
			mentions = append(mentions, *remote)
		}
	}
	return mentions, nil
}

func applyVisibilityAddressing(noteDoc map[string]any, visibility string, account *models.Account, mentions []models.Account) {
	public := "https://www.w3.org/ns/activitystreams#Public"
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

func (u UseCase) persistMentions(ctx context.Context, localAccountID string, noteID string, mentions []models.Account) *domainerrors.DomainError {
	for _, mention := range mentions {
		input := repos.CreateMentionInput{LocalAccountID: localAccountID, NoteID: noteID, AccountID: mention.ID, Username: mention.Username, Acct: mentionAcct(mention), URL: mentionURL(mention, u.cfg.Host)}
		if _, err := u.cfg.MentionsRepo.CreateMention(ctx, nil, input); err != nil {
			return domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
	}
	return nil
}

func mentionAcct(account models.Account) string {
	if account.Domain != nil && *account.Domain != "" {
		return account.Username + "@" + *account.Domain
	}
	return account.Username
}

func mentionURL(account models.Account, host string) string {
	if account.URL != nil && *account.URL != "" {
		return *account.URL
	}
	if account.Domain == nil {
		return strings.TrimRight(host, "/") + "/@" + account.Username
	}
	return account.URI
}

func (u UseCase) createLocalStatusNotifications(ctx context.Context, actor *models.Account, statusID string, mentions []models.Account) *domainerrors.DomainError {
	for _, mention := range mentions {
		if mention.Domain != nil || mention.ID == actor.ID {
			continue
		}
		if _, err := u.cfg.SocialRepo.CreateNotification(ctx, nil, mention.ID, actor.URI, "mention", &statusID); err != nil {
			return domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
	}
	return nil
}

func mentionInboxes(mentions []models.Account) []string {
	inboxes := make([]string, 0, len(mentions))
	seen := map[string]bool{}
	for _, mention := range mentions {
		if mention.Domain == nil || mention.InboxURI == "" || seen[mention.InboxURI] {
			continue
		}
		seen[mention.InboxURI] = true
		inboxes = append(inboxes, mention.InboxURI)
	}
	return inboxes
}

func normalizedVisibility(visibility string) string {
	switch visibility {
	case "public", "unlisted", "private", "direct":
		return visibility
	default:
		return "public"
	}
}
