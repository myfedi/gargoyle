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
	ListOutboxActivitiesPaged(tx *db.Tx, localAccountID string, limit int, offset int) ([]models.Activity, error)
	CountOutboxActivities(tx *db.Tx, localAccountID string) (int, error)
}

type CreateFollowInput struct {
	LocalAccountID string
	RemoteActor    string
	RemoteInbox    *string
	ActivityID     string
	Direction      string
}

type FollowsRepository interface {
	CreateFollow(tx *db.Tx, input CreateFollowInput) (*models.Follow, error)
	AcceptFollow(tx *db.Tx, followID string) error
	DeleteFollowByActor(tx *db.Tx, localAccountID string, remoteActor string) error
	ListFollowers(tx *db.Tx, localAccountID string) ([]models.Follow, error)
	ListFollowersPaged(tx *db.Tx, localAccountID string, limit int, offset int) ([]models.Follow, error)
	CountFollowers(tx *db.Tx, localAccountID string) (int, error)
	CreateFollowing(tx *db.Tx, input CreateFollowInput) (*models.Follow, error)
	AcceptFollowingByActor(tx *db.Tx, localAccountID string, remoteActor string) error
	RejectFollowingByActor(tx *db.Tx, localAccountID string, remoteActor string) error
	ListFollowing(tx *db.Tx, localAccountID string) ([]models.Follow, error)
}
