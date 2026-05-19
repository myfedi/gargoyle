package repos

import (
	"context"
	"errors"
	"time"

	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	"github.com/myfedi/gargoyle/domain/models"
	dbPorts "github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	dbUtils "github.com/myfedi/gargoyle/infrastructure/db"
	dbModels "github.com/myfedi/gargoyle/infrastructure/db/models"
	"github.com/uptrace/bun"
)

type ActivitiesRepo struct{ db bun.IDB }

func NewActivitiesRepo(db *bun.DB) *ActivitiesRepo { return &ActivitiesRepo{db: db} }

var _ repos.ActivitiesRepository = &ActivitiesRepo{}

func (r *ActivitiesRepo) CreateActivity(tx *dbPorts.Tx, input repos.CreateActivityInput) (*models.Activity, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}

	ulid, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}

	activity := &dbModels.Activity{
		ID:             ulid,
		LocalAccountID: input.LocalAccountID,
		Direction:      string(input.Direction),
		Type:           input.Type,
		Actor:          input.Actor,
		Object:         input.Object,
		RawJSON:        input.RawJSON,
	}
	_, err = db.NewInsert().Model(activity).Exec(context.Background())
	if err != nil {
		return nil, err
	}
	model := activity.ToModel()
	return &model, nil
}

func (r *ActivitiesRepo) ListOutboxActivities(tx *dbPorts.Tx, localAccountID string) ([]models.Activity, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}

	var activities []dbModels.Activity
	err = db.NewSelect().
		Model(&activities).
		Where("local_account_id = ?", localAccountID).
		Where("direction = ?", string(models.ActivityDirectionOutbox)).
		Order("created_at DESC").
		Scan(context.Background())
	if err != nil {
		return nil, err
	}

	res := make([]models.Activity, 0, len(activities))
	for _, activity := range activities {
		res = append(res, activity.ToModel())
	}
	return res, nil
}

type FollowsRepo struct{ db bun.IDB }

func NewFollowsRepo(db *bun.DB) *FollowsRepo { return &FollowsRepo{db: db} }

var _ repos.FollowsRepository = &FollowsRepo{}

func (r *FollowsRepo) CreateFollow(tx *dbPorts.Tx, input repos.CreateFollowInput) (*models.Follow, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}

	ulid, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}

	follow := &dbModels.Follow{
		ID:             ulid,
		LocalAccountID: input.LocalAccountID,
		RemoteActor:    input.RemoteActor,
		RemoteInbox:    input.RemoteInbox,
		ActivityID:     input.ActivityID,
	}
	_, err = db.NewInsert().Model(follow).Exec(context.Background())
	if err != nil {
		return nil, err
	}
	model := follow.ToModel()
	return &model, nil
}

func (r *FollowsRepo) AcceptFollow(tx *dbPorts.Tx, followID string) error {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	_, err = db.NewUpdate().
		Model((*dbModels.Follow)(nil)).
		Set("accepted_at = ?", now).
		Where("id = ?", followID).
		Exec(context.Background())
	return err
}

func (r *FollowsRepo) DeleteFollowByActor(tx *dbPorts.Tx, localAccountID string, remoteActor string) error {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return err
	}

	_, err = db.NewDelete().
		Model((*dbModels.Follow)(nil)).
		Where("local_account_id = ?", localAccountID).
		Where("remote_actor = ?", remoteActor).
		Exec(context.Background())
	return err
}

func (r *FollowsRepo) ListFollowers(tx *dbPorts.Tx, localAccountID string) ([]models.Follow, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}

	var follows []dbModels.Follow
	err = db.NewSelect().
		Model(&follows).
		Where("local_account_id = ?", localAccountID).
		Where("accepted_at IS NOT NULL").
		Order("created_at DESC").
		Scan(context.Background())
	if err != nil {
		return nil, err
	}

	res := make([]models.Follow, 0, len(follows))
	for _, follow := range follows {
		res = append(res, follow.ToModel())
	}
	return res, nil
}

func unwrapDB(defaultDB bun.IDB, tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return defaultDB, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New("internal error: unexpected tx implementation provided")
	}
	return adapted.Unwrap(), nil
}
