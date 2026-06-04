package repos

import (
	"context"

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
	CreateActivity(ctx context.Context, tx *db.Tx, input CreateActivityInput) (*models.Activity, error)
	ListOutboxActivities(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Activity, error)
	GetActivityByID(ctx context.Context, tx *db.Tx, id string) (*models.Activity, error)
	ListOutboxActivitiesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit, offset int) ([]models.Activity, error)
	ListPublicOutboxActivitiesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit, offset int) ([]models.Activity, error)
	CountOutboxActivities(ctx context.Context, tx *db.Tx, localAccountID string) (int, error)
	CountPublicOutboxActivities(ctx context.Context, tx *db.Tx, localAccountID string) (int, error)
}

type CreateFollowInput struct {
	LocalAccountID string
	RemoteActor    string
	RemoteInbox    *string
	ActivityID     string
	Direction      string
}

type FollowsRepository interface {
	CreateFollow(ctx context.Context, tx *db.Tx, input CreateFollowInput) (*models.Follow, error)
	AcceptFollow(ctx context.Context, tx *db.Tx, followID string) error
	AcceptFollowByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor string) (*models.Follow, error)
	GetFollowByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor, direction string) (*models.Follow, error)
	DeleteFollowByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor string) error
	ListFollowers(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Follow, error)
	ListFollowersPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit, offset int) ([]models.Follow, error)
	ListPendingFollowers(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Follow, error)
	CountFollowers(ctx context.Context, tx *db.Tx, localAccountID string) (int, error)
	CreateFollowing(ctx context.Context, tx *db.Tx, input CreateFollowInput) (*models.Follow, error)
	AcceptFollowingByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor string) error
	RejectFollowingByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor string) error
	DeleteFollowingByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor string) error
	ListFollowing(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Follow, error)
	ListFollowingIncludingPending(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Follow, error)
}
