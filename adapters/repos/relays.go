package repos

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	dbPorts "github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	dbUtils "github.com/myfedi/gargoyle/infrastructure/db"
	dbModels "github.com/myfedi/gargoyle/infrastructure/db/models"
	"github.com/uptrace/bun"
)

type RelaysRepo struct{ db bun.IDB }

func NewRelaysRepo(db *bun.DB) *RelaysRepo { return &RelaysRepo{db: db} }

var _ repos.RelaySubscriptionsRepository = &RelaysRepo{}

func (r *RelaysRepo) CreateRelaySubscription(ctx context.Context, tx *dbPorts.Tx, input repos.CreateRelaySubscriptionInput) (*models.RelaySubscription, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	row := &dbModels.RelaySubscription{ID: id, ActorURI: input.ActorURI, InboxURI: input.InboxURI, Status: models.RelayStatusPending, CreatedByUserID: input.CreatedByUserID, UpdatedAt: now}
	_, err = db.NewInsert().Model(row).
		On("CONFLICT (actor_uri) DO UPDATE").
		Set("inbox_uri = EXCLUDED.inbox_uri").
		Set("status = ?", models.RelayStatusPending).
		Set("created_by_user_id = EXCLUDED.created_by_user_id").
		Set("last_error = NULL").
		Set("updated_at = ?", now).
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetRelaySubscriptionByActor(ctx, tx, input.ActorURI)
}

func (r *RelaysRepo) ListRelaySubscriptions(ctx context.Context, tx *dbPorts.Tx) ([]models.RelaySubscription, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}
	var rows []dbModels.RelaySubscription
	if err := db.NewSelect().Model(&rows).Order("actor_uri ASC").Scan(ctx); err != nil {
		return nil, err
	}
	return relayRowsToModels(rows), nil
}

func (r *RelaysRepo) ListAcceptedRelaySubscriptions(ctx context.Context, tx *dbPorts.Tx) ([]models.RelaySubscription, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}
	var rows []dbModels.RelaySubscription
	if err := db.NewSelect().Model(&rows).Where("status = ?", models.RelayStatusAccepted).Order("actor_uri ASC").Scan(ctx); err != nil {
		return nil, err
	}
	return relayRowsToModels(rows), nil
}

func (r *RelaysRepo) GetRelaySubscriptionByActor(ctx context.Context, tx *dbPorts.Tx, actorURI string) (*models.RelaySubscription, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}
	var row dbModels.RelaySubscription
	if err := db.NewSelect().Model(&row).Where("actor_uri = ?", actorURI).Limit(1).Scan(ctx); err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}

func (r *RelaysRepo) GetRelaySubscriptionByID(ctx context.Context, tx *dbPorts.Tx, id string) (*models.RelaySubscription, error) {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return nil, err
	}
	var row dbModels.RelaySubscription
	if err := db.NewSelect().Model(&row).Where("id = ?", id).Limit(1).Scan(ctx); err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}

func (r *RelaysRepo) MarkRelaySubscriptionAccepted(ctx context.Context, tx *dbPorts.Tx, actorURI string, acceptedAt time.Time) error {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return err
	}
	_, err = db.NewUpdate().Model((*dbModels.RelaySubscription)(nil)).Set("status = ?", models.RelayStatusAccepted).Set("accepted_at = ?", acceptedAt).Set("updated_at = ?", time.Now().UTC()).Where("actor_uri = ?", actorURI).Exec(ctx)
	return err
}

func (r *RelaysRepo) DisableRelaySubscription(ctx context.Context, tx *dbPorts.Tx, actorURI string) error {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return err
	}
	_, err = db.NewUpdate().Model((*dbModels.RelaySubscription)(nil)).Set("status = ?", models.RelayStatusDisabled).Set("updated_at = ?", time.Now().UTC()).Where("actor_uri = ?", actorURI).Exec(ctx)
	return err
}

func (r *RelaysRepo) DeleteRelaySubscription(ctx context.Context, tx *dbPorts.Tx, actorURI string) error {
	db, err := unwrapDB(r.db, tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.RelaySubscription)(nil)).Where("actor_uri = ?", actorURI).Exec(ctx)
	return err
}

func relayRowsToModels(rows []dbModels.RelaySubscription) []models.RelaySubscription {
	res := make([]models.RelaySubscription, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.ToModel())
	}
	return res
}
