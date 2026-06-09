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

type PushRepo struct{ db bun.IDB }

func NewPushRepo(db *bun.DB) *PushRepo { return &PushRepo{db: db} }

var _ repos.PushSubscriptionRepository = &PushRepo{}
var _ repos.PushDeliveryJobsRepository = &PushRepo{}

func (r *PushRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New(unexpectedTxImplementationError)
	}
	return adapted.Unwrap(), nil
}

func pushRowFromInput(id string, input repos.UpsertPushSubscriptionInput) *dbModels.PushSubscription {
	return &dbModels.PushSubscription{ID: id, LocalAccountID: input.LocalAccountID, AccessTokenID: input.AccessTokenID, Endpoint: input.Endpoint, KeyP256DH: input.KeyP256DH, KeyAuth: input.KeyAuth, Policy: normalizePushPolicy(input.Policy), AlertMention: input.Alerts.Mention, AlertStatus: input.Alerts.Status, AlertReblog: input.Alerts.Reblog, AlertFollow: input.Alerts.Follow, AlertFollowRequest: input.Alerts.FollowRequest, AlertFavourite: input.Alerts.Favourite, AlertPoll: input.Alerts.Poll, AlertUpdate: input.Alerts.Update, AlertAdminSignUp: input.Alerts.AdminSignUp, AlertAdminReport: input.Alerts.AdminReport}
}

func normalizePushPolicy(policy string) string {
	if policy == "" {
		return "all"
	}
	return policy
}

func (r *PushRepo) UpsertPushSubscription(ctx context.Context, tx *dbPorts.Tx, input repos.UpsertPushSubscriptionInput) (*models.PushSubscription, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	row := pushRowFromInput(id, input)
	now := time.Now().UTC()
	_, err = db.NewInsert().Model(row).On("CONFLICT (access_token_id) DO UPDATE").Set("updated_at = ?", now).Set("local_account_id = EXCLUDED.local_account_id").Set("endpoint = EXCLUDED.endpoint").Set("key_p256dh = EXCLUDED.key_p256dh").Set("key_auth = EXCLUDED.key_auth").Set("policy = EXCLUDED.policy").Set("alert_mention = EXCLUDED.alert_mention").Set("alert_status = EXCLUDED.alert_status").Set("alert_reblog = EXCLUDED.alert_reblog").Set("alert_follow = EXCLUDED.alert_follow").Set("alert_follow_request = EXCLUDED.alert_follow_request").Set("alert_favourite = EXCLUDED.alert_favourite").Set("alert_poll = EXCLUDED.alert_poll").Set("alert_update = EXCLUDED.alert_update").Set("alert_admin_sign_up = EXCLUDED.alert_admin_sign_up").Set("alert_admin_report = EXCLUDED.alert_admin_report").Exec(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetPushSubscriptionByToken(ctx, tx, input.AccessTokenID)
}

func (r *PushRepo) GetPushSubscriptionByToken(ctx context.Context, tx *dbPorts.Tx, accessTokenID string) (*models.PushSubscription, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var row dbModels.PushSubscription
	if err := db.NewSelect().Model(&row).Where("access_token_id = ?", accessTokenID).Scan(ctx); err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}

func (r *PushRepo) UpdatePushSubscription(ctx context.Context, tx *dbPorts.Tx, accessTokenID, policy string, alerts models.PushAlerts) (*models.PushSubscription, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	_, err = db.NewUpdate().Model((*dbModels.PushSubscription)(nil)).Set("updated_at = ?", time.Now().UTC()).Set("policy = ?", normalizePushPolicy(policy)).Set("alert_mention = ?", alerts.Mention).Set("alert_status = ?", alerts.Status).Set("alert_reblog = ?", alerts.Reblog).Set("alert_follow = ?", alerts.Follow).Set("alert_follow_request = ?", alerts.FollowRequest).Set("alert_favourite = ?", alerts.Favourite).Set("alert_poll = ?", alerts.Poll).Set("alert_update = ?", alerts.Update).Set("alert_admin_sign_up = ?", alerts.AdminSignUp).Set("alert_admin_report = ?", alerts.AdminReport).Where("access_token_id = ?", accessTokenID).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetPushSubscriptionByToken(ctx, tx, accessTokenID)
}

func (r *PushRepo) DeletePushSubscriptionByToken(ctx context.Context, tx *dbPorts.Tx, accessTokenID string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.PushSubscription)(nil)).Where("access_token_id = ?", accessTokenID).Exec(ctx)
	return err
}

func (r *PushRepo) DeletePushSubscription(ctx context.Context, tx *dbPorts.Tx, id string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.PushSubscription)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func (r *PushRepo) ClaimDuePushDeliveryJobs(ctx context.Context, tx *dbPorts.Tx, now time.Time, limit int) ([]models.PushDeliveryJob, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 25
	}
	staleBefore := now.Add(-15 * time.Minute)
	var ids []string
	if err := db.NewSelect().Model((*dbModels.PushDeliveryJob)(nil)).Column("id").WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.WhereOr("status = ? AND next_attempt_at <= ?", string(models.JobStatusPending), now).WhereOr("status = ? AND updated_at <= ?", "processing", staleBefore)
	}).Order("next_attempt_at ASC").Limit(limit).Scan(ctx, &ids); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if _, err := db.NewUpdate().Model((*dbModels.PushDeliveryJob)(nil)).Set("status = ?", "processing").Set("updated_at = ?", now).Where("id IN (?)", bun.In(ids)).Exec(ctx); err != nil {
		return nil, err
	}
	var rows []dbModels.PushDeliveryJob
	if err := db.NewSelect().Model(&rows).Where("id IN (?)", bun.In(ids)).Order("next_attempt_at ASC").Scan(ctx); err != nil {
		return nil, err
	}
	out := make([]models.PushDeliveryJob, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ToModel())
	}
	return out, nil
}

func (r *PushRepo) MarkPushDeliveryJobDelivered(ctx context.Context, tx *dbPorts.Tx, id string, deliveredAt time.Time) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewUpdate().Model((*dbModels.PushDeliveryJob)(nil)).Set("status = ?", string(models.JobStatusDone)).Set("delivered_at = ?", deliveredAt).Set("updated_at = ?", deliveredAt).Where("id = ?", id).Exec(ctx)
	return err
}
func (r *PushRepo) MarkPushDeliveryJobFailed(ctx context.Context, tx *dbPorts.Tx, id string, attempts int, nextAttemptAt time.Time, lastError string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	status := string(models.JobStatusPending)
	if attempts >= 10 {
		status = string(models.JobStatusFailed)
	}
	_, err = db.NewUpdate().Model((*dbModels.PushDeliveryJob)(nil)).Set("status = ?", status).Set("attempts = ?", attempts).Set("next_attempt_at = ?", nextAttemptAt).Set("last_error = ?", lastError).Set("updated_at = ?", time.Now().UTC()).Where("id = ?", id).Exec(ctx)
	return err
}

func (r *PushRepo) GetPushDeliveryPayload(ctx context.Context, tx *dbPorts.Tx, job models.PushDeliveryJob) (*models.PushSubscription, *models.Notification, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, nil, err
	}
	var sub dbModels.PushSubscription
	if err := db.NewSelect().Model(&sub).Where("id = ?", job.SubscriptionID).Scan(ctx); err != nil {
		return nil, nil, err
	}
	var notification dbModels.Notification
	if err := db.NewSelect().Model(&notification).Where("id = ?", job.NotificationID).Scan(ctx); err != nil {
		return nil, nil, err
	}
	sm := sub.ToModel()
	nm := notification.ToModel()
	return &sm, &nm, nil
}
