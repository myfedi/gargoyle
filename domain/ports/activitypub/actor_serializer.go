package activitypub

import "github.com/myfedi/gargoyle/domain/models"

type ActorSerializer interface {
	Marshall(models.Account) (string, error)
	Unmarshall(string) (*models.Account, error)
}
