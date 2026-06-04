package activitypub

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	ap "github.com/go-ap/activitypub"
	"github.com/myfedi/gargoyle/domain/models"
	activitypub "github.com/myfedi/gargoyle/domain/ports/activitypub"
)

type ActorSerializerConfig struct{}
type ActorSerializer struct {
	cfg ActorSerializerConfig
}

var _ activitypub.ActorSerializer = ActorSerializer{}

func NewActorSerializer(cfg ActorSerializerConfig) ActorSerializer {
	return ActorSerializer{
		cfg: cfg,
	}
}

func (s ActorSerializer) Marshall(account models.Account) (string, error) {
	actor, err := mapActor(account)
	if err != nil {
		return "", err
	}

	data, err := actor.MarshalJSON()
	if err != nil {
		return "", err
	}

	data, err = ensureActorDocumentMetadata(data, account)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (s ActorSerializer) Unmarshall(input string) (*models.Account, error) {
	var account models.Account
	err := json.Unmarshal([]byte(input), &account)
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func mapActor(account models.Account) (*ap.Actor, error) {
	id := ap.ID(account.URI)

	var actor *ap.Actor
	switch account.ActorType {
	case models.ActorTypeUnknown:
		return nil, errors.New("can't handle unknown actor type")
	case models.ActorTypeApplication:
		actor = ap.ApplicationNew(id)
	case models.ActorTypeGroup:
		actor = ap.GroupNew(id)
	case models.ActorTypeOrganization:
		actor = ap.OrganizationNew(id)
	case models.ActorTypePerson:
		actor = ap.PersonNew(id)
	case models.ActorTypeService:
		actor = ap.ServiceNew(id)
	}

	actor.Name = ap.DefaultNaturalLanguage(firstNonEmpty(stringValue(account.DisplayName), account.Username))
	actor.PreferredUsername = ap.DefaultNaturalLanguage(account.Username)
	actor.Summary = ap.DefaultNaturalLanguage(stringValue(account.Summary))
	actor.Inbox = ap.IRI(account.InboxURI)
	if account.OutboxURI != nil {
		actor.Outbox = ap.IRI(*account.OutboxURI)
	}
	actor.Followers = ap.IRI(account.FollowersURI)
	actor.Following = ap.IRI(account.FollowingURI)
	if avatarURL := accountAvatarURL(account); avatarURL != "" {
		actor.Icon = ap.Image{Type: ap.ImageType, URL: ap.IRI(avatarURL)}
	}
	if headerURL := accountHeaderURL(account); headerURL != "" {
		actor.Image = ap.Image{Type: ap.ImageType, URL: ap.IRI(headerURL)}
	}
	actor.PublicKey = ap.PublicKey{
		ID:           ap.ID(id + "#main-key"),
		Owner:        ap.IRI(id),
		PublicKeyPem: account.PublicKey,
	}

	return actor, nil
}

func ensureActorDocumentMetadata(data []byte, account models.Account) ([]byte, error) {
	var actor map[string]json.RawMessage
	if err := json.Unmarshal(data, &actor); err != nil {
		return nil, err
	}

	if _, ok := actor[activityStreamsContextKey]; !ok {
		actor[activityStreamsContextKey] = json.RawMessage(`"` + activityStreamsContextURI + `"`)
	}
	if account.FeaturedCollectionURI != "" {
		featured, err := json.Marshal(account.FeaturedCollectionURI)
		if err != nil {
			return nil, err
		}
		actor["featured"] = featured
	}
	if sharedInbox := accountSharedInboxURL(account); sharedInbox != "" {
		endpoints, err := json.Marshal(map[string]string{"sharedInbox": sharedInbox})
		if err != nil {
			return nil, err
		}
		actor["endpoints"] = endpoints
	}
	actor["manuallyApprovesFollowers"] = json.RawMessage(boolJSON(account.Locked))

	return json.Marshal(actor)
}

func boolJSON(value bool) []byte {
	if value {
		return []byte("true")
	}
	return []byte("false")
}

func stringValue(str *string) string {
	if str == nil {
		return ""
	}
	return *str
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func accountAvatarURL(account models.Account) string {
	if account.AvatarURL != nil && *account.AvatarURL != "" {
		return *account.AvatarURL
	}
	if account.AvatarMediaID == nil || *account.AvatarMediaID == "" {
		return ""
	}
	return accountMediaURL(account.URI, *account.AvatarMediaID)
}

func accountHeaderURL(account models.Account) string {
	if account.HeaderURL != nil && *account.HeaderURL != "" {
		return *account.HeaderURL
	}
	if account.HeaderMediaID == nil || *account.HeaderMediaID == "" {
		return ""
	}
	return accountMediaURL(account.URI, *account.HeaderMediaID)
}

func accountSharedInboxURL(account models.Account) string {
	parsed, err := url.Parse(account.URI)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/inbox"
}

func accountMediaURL(actorURI, mediaID string) string {
	parsed, err := url.Parse(actorURI)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/media/" + strings.TrimLeft(mediaID, "/")
}
