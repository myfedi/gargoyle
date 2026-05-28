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

func (r *ActivitiesRepo) CreateActivity(ctx context.Context, tx *dbPorts.Tx, input repos.CreateActivityInput) (*models.Activity, error) {
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
	_, err = db.NewInsert().Model(activity).Exec(ctx)
	if err != nil {
		return nil, err
	}
	model := activity.ToModel()
	return &model, nil
}

func (r *ActivitiesRepo) ListOutboxActivities(ctx context.Context, tx *dbPorts.Tx, localAccountID string) ([]models.Activity, error) {
	return r.ListOutboxActivitiesPaged(ctx, tx, localAccountID, 0, 0)
}

func (r *ActivitiesRepo) CountOutboxActivities(ctx context.Context, tx *dbPorts.Tx, localAccountID string) (int, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return 0, err
	}
	return db.NewSelect().
		Model((*dbModels.Activity)(nil)).
		Where("local_account_id = ?", localAccountID).
		Where("direction = ?", string(models.ActivityDirectionOutbox)).
		Count(ctx)
}

func (r *ActivitiesRepo) ListOutboxActivitiesPaged(ctx context.Context, tx *dbPorts.Tx, localAccountID string, limit int, offset int) ([]models.Activity, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}

	var activities []dbModels.Activity
	query := db.NewSelect().
		Model(&activities).
		Where("local_account_id = ?", localAccountID).
		Where("direction = ?", string(models.ActivityDirectionOutbox)).
		Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}
	err = query.Scan(ctx)
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

func (r *FollowsRepo) CreateFollow(ctx context.Context, tx *dbPorts.Tx, input repos.CreateFollowInput) (*models.Follow, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}

	ulid, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}

	direction := input.Direction
	if direction == "" {
		direction = "follower"
	}
	follow := &dbModels.Follow{
		ID:             ulid,
		LocalAccountID: input.LocalAccountID,
		RemoteActor:    input.RemoteActor,
		RemoteInbox:    input.RemoteInbox,
		ActivityID:     input.ActivityID,
		Direction:      direction,
	}
	_, err = db.NewInsert().Model(follow).Exec(ctx)
	if err != nil {
		return nil, err
	}
	model := follow.ToModel()
	return &model, nil
}

func (r *FollowsRepo) AcceptFollow(ctx context.Context, tx *dbPorts.Tx, followID string) error {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	_, err = db.NewUpdate().
		Model((*dbModels.Follow)(nil)).
		Set("accepted_at = ?", now).
		Where("id = ?", followID).
		Exec(ctx)
	return err
}

func (r *FollowsRepo) CreateFollowing(ctx context.Context, tx *dbPorts.Tx, input repos.CreateFollowInput) (*models.Follow, error) {
	input.Direction = "following"
	return r.CreateFollow(ctx, tx, input)
}

func (r *FollowsRepo) AcceptFollowingByActor(ctx context.Context, tx *dbPorts.Tx, localAccountID string, remoteActor string) error {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = db.NewUpdate().Model((*dbModels.Follow)(nil)).
		Set("accepted_at = ?", now).
		Where("local_account_id = ?", localAccountID).
		Where("remote_actor = ?", remoteActor).
		Where("direction = ?", "following").
		Exec(ctx)
	return err
}

func (r *FollowsRepo) RejectFollowingByActor(ctx context.Context, tx *dbPorts.Tx, localAccountID string, remoteActor string) error {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.Follow)(nil)).
		Where("local_account_id = ?", localAccountID).
		Where("remote_actor = ?", remoteActor).
		Where("direction = ?", "following").
		Exec(ctx)
	return err
}

func (r *FollowsRepo) DeleteFollowingByActor(ctx context.Context, tx *dbPorts.Tx, localAccountID string, remoteActor string) error {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().
		Model((*dbModels.Follow)(nil)).
		Where("local_account_id = ?", localAccountID).
		Where("remote_actor = ?", remoteActor).
		Where("direction = ?", "following").
		Exec(ctx)
	return err
}

func (r *FollowsRepo) DeleteFollowByActor(ctx context.Context, tx *dbPorts.Tx, localAccountID string, remoteActor string) error {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return err
	}

	_, err = db.NewDelete().
		Model((*dbModels.Follow)(nil)).
		Where("local_account_id = ?", localAccountID).
		Where("remote_actor = ?", remoteActor).
		Where("direction = ?", "follower").
		Exec(ctx)
	return err
}

func (r *FollowsRepo) ListFollowers(ctx context.Context, tx *dbPorts.Tx, localAccountID string) ([]models.Follow, error) {
	return r.ListFollowersPaged(ctx, tx, localAccountID, 0, 0)
}

func (r *FollowsRepo) CountFollowers(ctx context.Context, tx *dbPorts.Tx, localAccountID string) (int, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return 0, err
	}
	return db.NewSelect().
		Model((*dbModels.Follow)(nil)).
		Where("local_account_id = ?", localAccountID).
		Where("direction = ?", "follower").
		Where("accepted_at IS NOT NULL").
		Count(ctx)
}

func (r *FollowsRepo) ListFollowersPaged(ctx context.Context, tx *dbPorts.Tx, localAccountID string, limit int, offset int) ([]models.Follow, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}

	var follows []dbModels.Follow
	query := db.NewSelect().
		Model(&follows).
		Where("local_account_id = ?", localAccountID).
		Where("direction = ?", "follower").
		Where("accepted_at IS NOT NULL").
		Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}
	err = query.Scan(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]models.Follow, 0, len(follows))
	for _, follow := range follows {
		res = append(res, follow.ToModel())
	}
	return res, nil
}

func (r *FollowsRepo) ListFollowing(ctx context.Context, tx *dbPorts.Tx, localAccountID string) ([]models.Follow, error) {
	return r.listFollowing(ctx, tx, localAccountID, true)
}

func (r *FollowsRepo) ListFollowingIncludingPending(ctx context.Context, tx *dbPorts.Tx, localAccountID string) ([]models.Follow, error) {
	return r.listFollowing(ctx, tx, localAccountID, false)
}

func (r *FollowsRepo) listFollowing(ctx context.Context, tx *dbPorts.Tx, localAccountID string, acceptedOnly bool) ([]models.Follow, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}
	var follows []dbModels.Follow
	query := db.NewSelect().Model(&follows).
		Where("local_account_id = ?", localAccountID).
		Where("direction = ?", "following").
		Order("created_at DESC")
	if acceptedOnly {
		query = query.Where("accepted_at IS NOT NULL")
	}
	err = query.Scan(ctx)
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
