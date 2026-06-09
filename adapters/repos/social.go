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

type SocialRepo struct{ db bun.IDB }

func NewSocialRepo(db *bun.DB) *SocialRepo { return &SocialRepo{db: db} }

var _ repos.SocialRepository = &SocialRepo{}

func (r *SocialRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New(unexpectedTxImplementationError)
	}
	return adapted.Unwrap(), nil
}

func (r *SocialRepo) CreateInteraction(ctx context.Context, tx *dbPorts.Tx, localAccountID, noteID, typ string) (*models.StatusInteraction, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	row := &dbModels.StatusInteraction{ID: id, LocalAccountID: localAccountID, NoteID: noteID, Type: typ}
	_, err = db.NewInsert().Model(row).On("CONFLICT DO NOTHING").Exec(ctx)
	if err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}
func (r *SocialRepo) DeleteInteraction(ctx context.Context, tx *dbPorts.Tx, localAccountID, noteID, typ string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.StatusInteraction)(nil)).Where("local_account_id = ?", localAccountID).Where("note_id = ?", noteID).Where("type = ?", typ).Exec(ctx) // NOSONAR
	return err
}
func (r *SocialRepo) InteractionExists(ctx context.Context, tx *dbPorts.Tx, localAccountID, noteID, typ string) (bool, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return false, err
	}
	return db.NewSelect().Model((*dbModels.StatusInteraction)(nil)).Where("local_account_id = ?", localAccountID).Where("note_id = ?", noteID).Where("type = ?", typ).Exists(ctx) // NOSONAR
}
func (r *SocialRepo) CountInteractionsForNote(ctx context.Context, tx *dbPorts.Tx, noteID, typ string) (int, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return 0, err
	}
	count, err := db.NewSelect().Model((*dbModels.StatusInteraction)(nil)).Where("note_id = ?", noteID).Where("type = ?", typ).Count(ctx) // NOSONAR
	if err != nil {
		return 0, err
	}
	return count, nil
}
func (r *SocialRepo) ListInteractions(ctx context.Context, tx *dbPorts.Tx, localAccountID, typ string, limit int) ([]models.StatusInteraction, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	var rows []dbModels.StatusInteraction
	if err := db.NewSelect().Model(&rows).Where("local_account_id = ?", localAccountID).Where("type = ?", typ).Order("created_at DESC").Limit(limit).Scan(ctx); err != nil { // NOSONAR
		return nil, err
	}
	res := make([]models.StatusInteraction, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.ToModel())
	}
	return res, nil
}

func (r *SocialRepo) CreateNotification(ctx context.Context, tx *dbPorts.Tx, localAccountID, actorAccountID, typ string, statusID *string) (*models.Notification, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	row := &dbModels.Notification{ID: id, LocalAccountID: localAccountID, ActorAccountID: actorAccountID, Type: typ, StatusID: statusID}
	_, err = db.NewInsert().Model(row).Exec(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.enqueuePushDeliveryJobs(ctx, db, row); err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}

func (r *SocialRepo) enqueuePushDeliveryJobs(ctx context.Context, db bun.IDB, notification *dbModels.Notification) error {
	var subscriptions []dbModels.PushSubscription
	query := db.NewSelect().Model(&subscriptions).Where("local_account_id = ?", notification.LocalAccountID)
	switch notification.Type {
	case "mention":
		query = query.Where("alert_mention = ?", true)
	case "status":
		query = query.Where("alert_status = ?", true)
	case "reblog":
		query = query.Where("alert_reblog = ?", true)
	case "follow":
		query = query.Where("alert_follow = ?", true)
	case "follow_request":
		query = query.Where("alert_follow_request = ?", true)
	case "favourite":
		query = query.Where("alert_favourite = ?", true)
	case "poll":
		query = query.Where("alert_poll = ?", true)
	case "update":
		query = query.Where("alert_update = ?", true)
	default:
		return nil
	}
	if err := query.Scan(ctx); err != nil {
		return err
	}
	for _, sub := range subscriptions {
		id, err := dbUtils.NewULID()
		if err != nil {
			return err
		}
		job := &dbModels.PushDeliveryJob{ID: id, SubscriptionID: sub.ID, NotificationID: notification.ID, NextAttemptAt: time.Now().UTC(), Status: string(models.JobStatusPending)}
		if _, err := db.NewInsert().Model(job).On("CONFLICT DO NOTHING").Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *SocialRepo) ListNotifications(ctx context.Context, tx *dbPorts.Tx, localAccountID string, limit int) ([]models.Notification, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	var rows []dbModels.Notification
	if err := db.NewSelect().Model(&rows).Where("local_account_id = ?", localAccountID).Order("created_at DESC").Limit(limit).Scan(ctx); err != nil { // NOSONAR
		return nil, err
	}
	res := make([]models.Notification, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.ToModel())
	}
	return res, nil
}
func (r *SocialRepo) DeleteNotification(ctx context.Context, tx *dbPorts.Tx, localAccountID, notificationID string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.Notification)(nil)).Where("local_account_id = ?", localAccountID).Where("id = ?", notificationID).Exec(ctx) // NOSONAR
	return err
}

func (r *SocialRepo) DeleteNotificationsByActorAndType(ctx context.Context, tx *dbPorts.Tx, localAccountID, actorAccountID, typ string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.Notification)(nil)).Where("local_account_id = ?", localAccountID).Where("actor_account_id = ?", actorAccountID).Where("type = ?", typ).Exec(ctx) // NOSONAR
	return err
}

func (r *SocialRepo) ClearNotifications(ctx context.Context, tx *dbPorts.Tx, localAccountID string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.Notification)(nil)).Where("local_account_id = ?", localAccountID).Exec(ctx) // NOSONAR
	return err
}
