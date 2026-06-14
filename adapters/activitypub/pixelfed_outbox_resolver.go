package activitypub

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
)

// PixelfedOutboxResolver adapts Pixelfed's public profile API for instances
// whose ActivityPub outbox only exposes totalItems without pages.
type PixelfedOutboxResolver struct {
	client *http.Client
}

func NewPixelfedOutboxResolver(client *http.Client) *PixelfedOutboxResolver {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &PixelfedOutboxResolver{client: client}
}

func (r *PixelfedOutboxResolver) ResolveOutboxPage(ctx context.Context, _ models.Account, pageURI, expectedActor string) ([]json.RawMessage, string, error) {
	parsed, err := url.Parse(pageURI)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || !strings.HasSuffix(parsed.Path, "/outbox") {
		return nil, "", nil
	}
	username := pixelfedOutboxUsername(parsed.Path)
	if username == "" {
		return nil, "", nil
	}
	profileID, err := r.profileID(ctx, parsed.Scheme+"://"+parsed.Host+"/"+username)
	if err != nil || profileID == "" {
		return nil, "", err
	}
	maxID := strings.TrimSpace(parsed.Query().Get("max_id"))
	apiURL := parsed.Scheme + "://" + parsed.Host + "/api/pixelfed/v1/accounts/" + url.PathEscape(profileID) + "/statuses?media_type=photo&limit=10"
	if maxID != "" {
		apiURL += "&max_id=" + url.QueryEscape(maxID)
	}
	statuses, err := r.statuses(ctx, apiURL)
	if err != nil || len(statuses) == 0 {
		return nil, "", err
	}
	items := make([]json.RawMessage, 0, len(statuses))
	for _, status := range statuses {
		raw, err := pixelfedStatusCreate(status, expectedActor)
		if err == nil && len(raw) > 0 {
			items = append(items, raw)
		}
	}
	next := ""
	if len(statuses) == 10 {
		if id := strings.TrimSpace(statuses[len(statuses)-1].ID); id != "" {
			nextURL := *parsed
			q := nextURL.Query()
			q.Set("max_id", id)
			nextURL.RawQuery = q.Encode()
			next = nextURL.String()
		}
	}
	return items, next, nil
}

func pixelfedOutboxUsername(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 3 && parts[0] == "users" && parts[2] == "outbox" {
		return parts[1]
	}
	return ""
}

var pixelfedProfileIDPattern = regexp.MustCompile(`<profile[^>]+profile-id="([0-9]+)"`)

func (r *PixelfedOutboxResolver) profileID(ctx context.Context, profileURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Gargoyle/1.0")
	req.Header.Set("Accept", "text/html")
	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("pixelfed profile fetch failed: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return "", err
	}
	match := pixelfedProfileIDPattern.FindSubmatch(body)
	if len(match) < 2 {
		return "", nil
	}
	return string(match[1]), nil
}

type pixelfedStatus struct {
	ID               string                    `json:"id"`
	URI              string                    `json:"uri"`
	URL              string                    `json:"url"`
	Content          string                    `json:"content"`
	CreatedAt        string                    `json:"created_at"`
	Visibility       string                    `json:"visibility"`
	Sensitive        bool                      `json:"sensitive"`
	SpoilerText      string                    `json:"spoiler_text"`
	MediaAttachments []pixelfedMediaAttachment `json:"media_attachments"`
}

type pixelfedMediaAttachment struct {
	Type        string `json:"type"`
	URL         string `json:"url"`
	PreviewURL  string `json:"preview_url"`
	Description string `json:"description"`
	Mime        string `json:"mime"`
}

func (r *PixelfedOutboxResolver) statuses(ctx context.Context, apiURL string) ([]pixelfedStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Gargoyle/1.0")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pixelfed statuses fetch failed: HTTP %d", resp.StatusCode)
	}
	var statuses []pixelfedStatus
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&statuses); err != nil {
		return nil, err
	}
	return statuses, nil
}

func pixelfedStatusCreate(status pixelfedStatus, actor string) (json.RawMessage, error) {
	if actor == "" || status.URI == "" {
		return nil, nil
	}
	published := pixelfedPublished(status.CreatedAt)
	note := map[string]any{
		"id":           status.URI,
		"type":         "Note",
		"attributedTo": actor,
		"content":      status.Content,
		"published":    published,
		"to":           []string{"https://www.w3.org/ns/activitystreams#Public"},
		"cc":           []string{strings.TrimRight(actor, "/") + "/followers"},
		"url":          status.URL,
		"sensitive":    status.Sensitive,
		"summary":      status.SpoilerText,
		"attachment":   pixelfedAttachments(status.MediaAttachments),
	}
	createID := status.URI + "#create"
	if id := strings.TrimSpace(status.ID); id != "" {
		createID = strings.TrimRight(actor, "/") + "/statuses/" + id + "/activity"
	}
	create := map[string]any{
		"id":        createID,
		"type":      "Create",
		"actor":     actor,
		"published": published,
		"to":        []string{"https://www.w3.org/ns/activitystreams#Public"},
		"cc":        []string{strings.TrimRight(actor, "/") + "/followers"},
		"object":    note,
	}
	return json.Marshal(create)
}

func pixelfedPublished(value string) string {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.Format(time.RFC3339)
	}
	return value
}

func pixelfedAttachments(media []pixelfedMediaAttachment) []map[string]any {
	attachments := make([]map[string]any, 0, len(media))
	for _, item := range media {
		mediaURL := strings.TrimSpace(item.URL)
		if mediaURL == "" {
			mediaURL = strings.TrimSpace(item.PreviewURL)
		}
		if mediaURL == "" {
			continue
		}
		mediaType := strings.TrimSpace(item.Mime)
		if mediaType == "" {
			mediaType = pixelfedMediaType(item.Type)
		}
		attachments = append(attachments, map[string]any{
			"type":      "Document",
			"mediaType": mediaType,
			"url":       mediaURL,
			"name":      html.EscapeString(strings.TrimSpace(item.Description)),
		})
	}
	return attachments
}

func pixelfedMediaType(kind string) string {
	switch strings.ToLower(kind) {
	case "image", "photo":
		return "image/jpeg"
	case "video":
		return "video/mp4"
	default:
		if _, err := strconv.Atoi(kind); err == nil {
			return "image/jpeg"
		}
		return "application/octet-stream"
	}
}
