package activitypub

import (
	"encoding/json"
	"errors"

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

	data, err = ensureActivityStreamsContext(data)
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

	actor.Name = ap.DefaultNaturalLanguage(account.Username)
	actor.PreferredUsername = ap.DefaultNaturalLanguage(account.Username)
	actor.Summary = ap.DefaultNaturalLanguage(stringValue(account.Summary))
	actor.Inbox = ap.IRI(account.InboxURI)
	if account.OutboxURI != nil {
		actor.Outbox = ap.IRI(*account.OutboxURI)
	}
	actor.Followers = ap.IRI(account.FollowersURI)
	actor.Following = ap.IRI(account.FollowingURI)
	actor.PublicKey = ap.PublicKey{
		ID:           ap.ID(id + "#main-key"),
		Owner:        ap.IRI(id),
		PublicKeyPem: account.PublicKey,
	}

	return actor, nil
}

func ensureActivityStreamsContext(data []byte) ([]byte, error) {
	var actor map[string]json.RawMessage
	if err := json.Unmarshal(data, &actor); err != nil {
		return nil, err
	}

	if _, ok := actor["@context"]; !ok {
		actor["@context"] = json.RawMessage(`"https://www.w3.org/ns/activitystreams"`)
	}

	return json.Marshal(actor)
}

func stringValue(str *string) string {
	if str == nil {
		return ""
	}
	return *str
}
