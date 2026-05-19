package models

import (
	"testing"

	domainmodels "github.com/myfedi/gargoyle/domain/models"
)

func TestAccountToModelMapsIntegerActorType(t *testing.T) {
	account := Account{ActorType: int(domainmodels.ActorTypePerson)}

	model := account.ToModel()

	if model.ActorType != domainmodels.ActorTypePerson {
		t.Fatalf("expected actor type %v, got %v", domainmodels.ActorTypePerson, model.ActorType)
	}
}
