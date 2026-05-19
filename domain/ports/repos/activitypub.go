package repos

import (
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type CreateActivityInput struct {
	LocalAccountID string
	Direction      models.ActivityDirection
	Type           string
	Actor          string
	Object         string
	RawJSON        string
}

type ActivitiesRepository interface {
	CreateActivity(tx *db.Tx, input CreateActivityInput) (*models.Activity, error)
	ListOutboxActivities(tx *db.Tx, localAccountID string) ([]models.Activity, error)
}

type CreateFollowInput struct {
	LocalAccountID string
	RemoteActor    string
	RemoteInbox    *string
	ActivityID     string
}

type FollowsRepository interface {
	CreateFollow(tx *db.Tx, input CreateFollowInput) (*models.Follow, error)
	AcceptFollow(tx *db.Tx, followID string) error
	ListFollowers(tx *db.Tx, localAccountID string) ([]models.Follow, error)
}
