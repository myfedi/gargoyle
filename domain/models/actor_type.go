package models

import "strings"

// ActorType represents the ActivityStreams actor type.
type ActorType int

const (
	ActorTypeUnknown ActorType = iota
	ActorTypeApplication
	ActorTypeGroup
	ActorTypeOrganization
	ActorTypePerson
	ActorTypeService
)

var actorTypeToStr = map[ActorType]string{
	ActorTypeApplication:  "application",
	ActorTypeGroup:        "group",
	ActorTypeOrganization: "organization",
	ActorTypePerson:       "person",
	ActorTypeService:      "service",
}

var strToActorType = map[string]ActorType{
	"application":  ActorTypeApplication,
	"group":        ActorTypeGroup,
	"organization": ActorTypeOrganization,
	"person":       ActorTypePerson,
	"service":      ActorTypeService,
}

func (t ActorType) String() string {
	if s, ok := actorTypeToStr[t]; ok {
		return s
	}
	return "unknown"
}

func ParseActorType(s string) ActorType {
	if t, ok := strToActorType[strings.ToLower(s)]; ok {
		return t
	}
	return ActorTypeUnknown
}
