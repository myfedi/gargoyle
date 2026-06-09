package clientapi

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
)

type InstanceInfo struct {
	Host          string
	Domain        string
	Title         string
	Description   string
	ServerVersion string
}

type CreateStatusInput struct {
	Content       string
	InReplyToID   string
	Visibility    string
	Sensitive     bool
	SpoilerText   string
	MediaIDs      []string
	ObjectType    string
	PollOptions   []string
	PollMultiple  bool
	PollExpiresIn int
}

type CreateStatusResult struct {
	Note            models.Note
	Account         models.Account
	Media           []models.MediaAttachment
	Mentions        []models.Mention
	RawJSON         []byte
	FollowerInboxes []string
	MentionInboxes  []string
}

type UpdateStatusInput struct {
	Content       string
	Visibility    string
	Sensitive     bool
	SpoilerText   string
	MediaIDs      []string
	ObjectType    string
	PollOptions   []string
	PollMultiple  bool
	PollExpiresIn int
}

type UpdateStatusResult struct {
	Note            models.Note
	Account         models.Account
	Media           []models.MediaAttachment
	Mentions        []models.Mention
	RawJSON         []byte
	FollowerInboxes []string
	MentionInboxes  []string
}

type TimelineItem struct {
	ID                 string
	URI                string
	CreatedAt          time.Time
	Note               models.Note
	Account            models.Account
	InReplyToAccountID *string
	Media              []models.MediaAttachment
	Mentions           []models.Mention
	Poll               *models.Poll
	Reblog             *TimelineItem
	Reblogged          bool
	Favourited         bool
	Bookmarked         bool
	Pinned             bool
	ReblogsCount       int
}

type TimelineOptions struct {
	Limit      int
	MaxID      string
	LocalOnly  bool
	RemoteOnly bool
}

type FollowAccountResult struct {
	Account models.Account
	RawJSON []byte
	Inbox   string
}

type Relationship struct {
	ID        string
	Following bool
	Requested bool
}

type UpdateCredentialsInput struct {
	DisplayName string
	Note        string
	Fields      []models.AccountProfileField
	Avatar      *UploadMediaInput
	Header      *UploadMediaInput
	Locked      bool
}

type UpdateCredentialsResult struct {
	Account         models.Account
	RawJSON         []byte
	FollowerInboxes []string
}
