package activitypub

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports"
)

const activityStreamsPublicURI = "https://www.w3.org/ns/activitystreams#Public"

type LemmyThreadResolver struct {
	fetcher ports.RemoteObjectFetcher
}

func NewLemmyThreadResolver(fetcher ports.RemoteObjectFetcher) LemmyThreadResolver {
	return LemmyThreadResolver{fetcher: fetcher}
}

func (r LemmyThreadResolver) ResolveReplies(ctx context.Context, objectURI string, signer *models.Account) ([]ports.RemoteReply, error) {
	if r.fetcher == nil {
		return nil, nil
	}
	apiURI, ok := lemmyCommentListURI(objectURI)
	if !ok {
		return nil, nil
	}
	raw, err := r.fetcher.FetchObject(ctx, apiURI, signer)
	if err != nil {
		return nil, nil
	}
	var response lemmyCommentListResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, nil
	}
	commentURIs := make(map[string]string, len(response.Comments))
	for _, item := range response.Comments {
		if item.Comment.ID > 0 && item.Comment.APID != "" {
			commentURIs[strconv.FormatInt(item.Comment.ID, 10)] = item.Comment.APID
		}
	}
	replies := make([]ports.RemoteReply, 0, len(response.Comments))
	for _, item := range response.Comments {
		if item.Comment.Removed || item.Comment.Deleted || item.Comment.APID == "" || item.Creator.ActorID == "" {
			continue
		}
		published, _ := time.Parse(time.RFC3339Nano, item.Comment.Published)
		reply := ports.RemoteReply{
			URI:          item.Comment.APID,
			AttributedTo: item.Creator.ActorID,
			Content:      item.Comment.Content,
			PublishedAt:  published,
			InReplyToURI: lemmyCommentParentURI(item.Comment.Path, objectURI, commentURIs),
			To:           []string{activityStreamsPublicURI},
		}
		if item.Community.ActorID != "" {
			reply.CC = []string{item.Community.ActorID}
		}
		replies = append(replies, reply)
	}
	return replies, nil
}

type lemmyCommentListResponse struct {
	Comments []struct {
		Comment struct {
			ID        int64  `json:"id"`
			Content   string `json:"content"`
			Removed   bool   `json:"removed"`
			Deleted   bool   `json:"deleted"`
			Published string `json:"published"`
			APID      string `json:"ap_id"`
			Path      string `json:"path"`
		} `json:"comment"`
		Creator struct {
			ActorID string `json:"actor_id"`
		} `json:"creator"`
		Community struct {
			ActorID string `json:"actor_id"`
		} `json:"community"`
	} `json:"comments"`
}

func lemmyCommentListURI(objectURI string) (string, bool) {
	parsed, err := url.Parse(objectURI)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "post" || parts[1] == "" {
		return "", false
	}
	for _, r := range parts[1] {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	api := *parsed
	api.Path = "/api/v3/comment/list"
	api.RawQuery = url.Values{"post_id": {parts[1]}, "type_": {"All"}, "sort": {"New"}, "max_depth": {"8"}, "limit": {"50"}}.Encode()
	return api.String(), true
}

func lemmyCommentParentURI(path, postURI string, commentURIs map[string]string) string {
	parts := strings.Split(path, ".")
	if len(parts) < 3 {
		return postURI
	}
	if uri := commentURIs[parts[len(parts)-2]]; uri != "" {
		return uri
	}
	return postURI
}
