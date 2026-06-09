package clientapi

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/domain/models"
	clientapiUC "github.com/myfedi/gargoyle/domain/usecases/clientapi"
)

type mentionResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Acct     string `json:"acct"`
	URL      string `json:"url"`
}

type pollOptionResponse struct {
	Title      string `json:"title"`
	VotesCount int    `json:"votes_count"`
}

type pollResponse struct {
	ID          string               `json:"id"`
	ExpiresAt   *string              `json:"expires_at"`
	Expired     bool                 `json:"expired"`
	Multiple    bool                 `json:"multiple"`
	VotesCount  int                  `json:"votes_count"`
	VotersCount int                  `json:"voters_count"`
	Voted       bool                 `json:"voted"`
	OwnVotes    []int                `json:"own_votes"`
	Options     []pollOptionResponse `json:"options"`
	Emojis      []any                `json:"emojis"`
}

type tagResponse struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type emojiResponse struct {
	Shortcode       string `json:"shortcode"`
	URL             string `json:"url"`
	StaticURL       string `json:"static_url"`
	VisibleInPicker bool   `json:"visible_in_picker"`
}

type statusResponse struct {
	ID                 string                    `json:"id"`
	URI                string                    `json:"uri"`
	URL                string                    `json:"url"`
	CreatedAt          string                    `json:"created_at"`
	EditedAt           *string                   `json:"edited_at"`
	Account            accountResponse           `json:"account"`
	Content            string                    `json:"content"`
	Visibility         string                    `json:"visibility"`
	ActivityPubType    string                    `json:"activitypub_type"`
	InReplyToID        *string                   `json:"in_reply_to_id"`
	InReplyToAccountID *string                   `json:"in_reply_to_account_id"`
	Sensitive          bool                      `json:"sensitive"`
	SpoilerText        string                    `json:"spoiler_text"`
	MediaAttachments   []mediaAttachmentResponse `json:"media_attachments"`
	Mentions           []mentionResponse         `json:"mentions"`
	Tags               []tagResponse             `json:"tags"`
	Emojis             []emojiResponse           `json:"emojis"`
	RepliesCount       int                       `json:"replies_count"`
	ReblogsCount       int                       `json:"reblogs_count"`
	FavouritesCount    int                       `json:"favourites_count"`
	Favourited         bool                      `json:"favourited"`
	Reblogged          bool                      `json:"reblogged"`
	Muted              bool                      `json:"muted"`
	Bookmarked         bool                      `json:"bookmarked"`
	Pinned             bool                      `json:"pinned"`
	Reblog             *statusResponse           `json:"reblog"`
	Poll               *pollResponse             `json:"poll"`
}

func timelineOptions(c *fiber.Ctx) clientapiUC.TimelineOptions {
	return clientapiUC.TimelineOptions{Limit: c.QueryInt("limit"), MaxID: c.Query("max_id"), LocalOnly: c.QueryBool("local"), RemoteOnly: c.QueryBool("remote")}
}

func timelineItemsToStatuses(items []clientapiUC.TimelineItem) []statusResponse {
	statuses := make([]statusResponse, 0, len(items))
	for _, item := range items {
		status := noteToStatus(item.Note, &item.Account)
		if item.ID != "" {
			status.ID = item.ID
		}
		if item.URI != "" {
			status.URI = item.URI
			status.URL = item.URI
		}
		if !item.CreatedAt.IsZero() {
			status.CreatedAt = item.CreatedAt.UTC().Format(time.RFC3339)
		}
		status.InReplyToAccountID = item.InReplyToAccountID
		status.MediaAttachments = mediaResponses(item.Media)
		status.Mentions = mentionResponses(item.Mentions)
		status.ReblogsCount = item.ReblogsCount
		status.FavouritesCount = item.FavouritesCount
		status.Reblogged = item.Reblogged
		status.Favourited = item.Favourited
		status.Bookmarked = item.Bookmarked
		status.Pinned = item.Pinned
		if item.Poll != nil {
			poll := pollToResponse(*item.Poll)
			status.Poll = &poll
		}
		if item.Reblog != nil {
			reblog := timelineItemsToStatuses([]clientapiUC.TimelineItem{*item.Reblog})[0]
			status.Content = ""
			status.MediaAttachments = []mediaAttachmentResponse{}
			status.Reblog = &reblog
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func tagResponses(tags []string) []tagResponse {
	res := make([]tagResponse, 0, len(tags))
	for _, tag := range tags {
		name := strings.TrimPrefix(tag, "#")
		if name == "" {
			continue
		}
		res = append(res, tagResponse{Name: name, URL: "/tags/" + name})
	}
	return res
}

func emojiResponses(emojis []models.CustomEmoji) []emojiResponse {
	res := make([]emojiResponse, 0, len(emojis))
	for _, emoji := range emojis {
		if emoji.Shortcode == "" || emoji.URL == "" {
			continue
		}
		staticURL := emoji.StaticURL
		if staticURL == "" {
			staticURL = emoji.URL
		}
		res = append(res, emojiResponse{Shortcode: emoji.Shortcode, URL: emoji.URL, StaticURL: staticURL, VisibleInPicker: false})
	}
	return res
}

func mentionResponses(mentions []models.Mention) []mentionResponse {
	res := make([]mentionResponse, 0, len(mentions))
	for _, mention := range mentions {
		res = append(res, mentionResponse{ID: mention.AccountID, Username: mention.Username, Acct: mention.Acct, URL: mention.URL})
	}
	return res
}

func pollToResponse(poll models.Poll) pollResponse {
	var expiresAt *string
	expired := false
	if poll.ExpiresAt != nil {
		formatted := poll.ExpiresAt.UTC().Format(time.RFC3339)
		expiresAt = &formatted
		expired = time.Now().UTC().After(*poll.ExpiresAt)
	}
	options := make([]pollOptionResponse, 0, len(poll.Options))
	votes := 0
	for _, option := range poll.Options {
		options = append(options, pollOptionResponse{Title: option.Title, VotesCount: option.VotesCount})
		votes += option.VotesCount
	}
	return pollResponse{ID: poll.NoteID, ExpiresAt: expiresAt, Expired: expired, Multiple: poll.Multiple, VotesCount: votes, VotersCount: votes, Voted: poll.Voted, OwnVotes: poll.OwnVotes, Options: options, Emojis: []any{}}
}

func normalizedResponseObjectType(objectType string) string {
	switch objectType {
	case "Article", "Page", "Question":
		return objectType
	default:
		return "Note"
	}
}

func noteToStatus(note models.Note, account *models.Account) statusResponse {
	created := note.PublishedAt
	if created.IsZero() {
		created = note.CreatedAt
	}
	if created.IsZero() {
		created = time.Now().UTC()
	}
	visibility := note.Visibility
	if visibility == "" {
		visibility = "public"
	}
	var editedAt *string
	if note.EditedAt != nil {
		formatted := note.EditedAt.UTC().Format(time.RFC3339)
		editedAt = &formatted
	}
	objectType := normalizedResponseObjectType(note.ObjectType)
	return statusResponse{ID: note.ID, URI: note.URI, URL: note.URI, CreatedAt: created.UTC().Format(time.RFC3339), EditedAt: editedAt, Account: accountToResponse(account), Content: note.Content, Visibility: visibility, ActivityPubType: objectType, InReplyToID: note.InReplyToID, Sensitive: note.Sensitive, SpoilerText: note.SpoilerText, MediaAttachments: []mediaAttachmentResponse{}, Mentions: []mentionResponse{}, Tags: tagResponses(note.Hashtags), Emojis: emojiResponses(note.Emojis)}
}
