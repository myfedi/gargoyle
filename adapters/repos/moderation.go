package repos

import (
	"context"
	"errors"
	"strings"
	"time"

	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	"github.com/myfedi/gargoyle/domain/models"
	dbPorts "github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	dbUtils "github.com/myfedi/gargoyle/infrastructure/db"
	dbModels "github.com/myfedi/gargoyle/infrastructure/db/models"
	"github.com/uptrace/bun"
)

type ModerationRepo struct{ db bun.IDB }

func NewModerationRepo(db *bun.DB) *ModerationRepo { return &ModerationRepo{db: db} }

var _ repos.DomainBlocksRepository = &ModerationRepo{}
var _ repos.ModerationJobsRepository = &ModerationRepo{}
var _ repos.DomainPurgeRepository = &ModerationRepo{}

func (r *ModerationRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New(unexpectedTxImplementationError)
	}
	return adapted.Unwrap(), nil
}

func (r *ModerationRepo) CreateDomainBlock(ctx context.Context, tx *dbPorts.Tx, input repos.CreateDomainBlockInput) (*models.DomainBlock, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	severity := input.Severity
	if severity == "" {
		severity = models.DomainBlockSeveritySuspend
	}
	now := time.Now().UTC()
	row := &dbModels.DomainBlock{ID: id, Domain: normalizeDomain(input.Domain), Severity: severity, RejectMedia: input.RejectMedia, PublicComment: input.PublicComment, PrivateComment: input.PrivateComment, CreatedByUserID: input.CreatedByUserID, UpdatedAt: now}
	_, err = db.NewInsert().Model(row).
		On("CONFLICT (domain) DO UPDATE").
		Set("severity = EXCLUDED.severity").
		Set("reject_media = EXCLUDED.reject_media").
		Set("public_comment = EXCLUDED.public_comment").
		Set("private_comment = EXCLUDED.private_comment").
		Set("created_by_user_id = EXCLUDED.created_by_user_id").
		Set("updated_at = ?", now).
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetDomainBlock(ctx, tx, input.Domain)
}

func (r *ModerationRepo) DeleteDomainBlock(ctx context.Context, tx *dbPorts.Tx, domain string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*dbModels.DomainBlock)(nil)).Where("domain = ?", normalizeDomain(domain)).Exec(ctx)
	return err
}

func (r *ModerationRepo) ListDomainBlocks(ctx context.Context, tx *dbPorts.Tx) ([]models.DomainBlock, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var rows []dbModels.DomainBlock
	if err := db.NewSelect().Model(&rows).Order("domain ASC").Scan(ctx); err != nil {
		return nil, err
	}
	res := make([]models.DomainBlock, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.ToModel())
	}
	return res, nil
}

func (r *ModerationRepo) GetDomainBlock(ctx context.Context, tx *dbPorts.Tx, domain string) (*models.DomainBlock, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var row dbModels.DomainBlock
	if err := db.NewSelect().Model(&row).Where("domain = ?", normalizeDomain(domain)).Limit(1).Scan(ctx); err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}

func (r *ModerationRepo) DomainIsSuspended(ctx context.Context, tx *dbPorts.Tx, domain string) (bool, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return false, err
	}
	return db.NewSelect().Model((*dbModels.DomainBlock)(nil)).Where("domain = ?", normalizeDomain(domain)).Where("severity = ?", models.DomainBlockSeveritySuspend).Exists(ctx)
}

func (r *ModerationRepo) CreateModerationJob(ctx context.Context, tx *dbPorts.Tx, input repos.CreateModerationJobInput) (*models.ModerationJob, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	id, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}
	next := input.NextAttemptAt
	if next.IsZero() {
		next = time.Now().UTC()
	}
	row := &dbModels.ModerationJob{ID: id, Kind: input.Kind, Payload: input.Payload, NextAttemptAt: next, Status: string(models.JobStatusPending)}
	if _, err := db.NewInsert().Model(row).Exec(ctx); err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}

func (r *ModerationRepo) ClaimDueModerationJobs(ctx context.Context, tx *dbPorts.Tx, now time.Time, limit int) ([]models.ModerationJob, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	staleBefore := now.Add(-15 * time.Minute)
	var ids []string
	if err := db.NewSelect().Model((*dbModels.ModerationJob)(nil)).Column("id").WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.WhereOr("status = ? AND next_attempt_at <= ?", string(models.JobStatusPending), now).WhereOr("status = ? AND updated_at <= ?", "processing", staleBefore)
	}).Order("next_attempt_at ASC").Limit(limit).Scan(ctx, &ids); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if _, err := db.NewUpdate().Model((*dbModels.ModerationJob)(nil)).Set("status = ?", "processing").Set("updated_at = ?", now).Where("id IN (?)", bun.In(ids)).Exec(ctx); err != nil {
		return nil, err
	}
	var rows []dbModels.ModerationJob
	if err := db.NewSelect().Model(&rows).Where("id IN (?)", bun.In(ids)).Order("next_attempt_at ASC").Scan(ctx); err != nil {
		return nil, err
	}
	jobs := make([]models.ModerationJob, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, row.ToModel())
	}
	return jobs, nil
}

func (r *ModerationRepo) MarkModerationJobDone(ctx context.Context, tx *dbPorts.Tx, id string, finishedAt time.Time) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	_, err = db.NewUpdate().Model((*dbModels.ModerationJob)(nil)).Set("status = ?", string(models.JobStatusDone)).Set("finished_at = ?", finishedAt).Set("updated_at = ?", finishedAt).Where("id = ?", id).Exec(ctx)
	return err
}

func (r *ModerationRepo) MarkModerationJobFailed(ctx context.Context, tx *dbPorts.Tx, id string, attempts int, nextAttemptAt time.Time, lastError string) error {
	db, err := r.resolveDB(tx)
	if err != nil {
		return err
	}
	status := string(models.JobStatusPending)
	if attempts >= 5 {
		status = string(models.JobStatusFailed)
	}
	_, err = db.NewUpdate().Model((*dbModels.ModerationJob)(nil)).Set("status = ?", status).Set("attempts = ?", attempts).Set("next_attempt_at = ?", nextAttemptAt).Set("last_error = ?", lastError).Set("updated_at = ?", time.Now().UTC()).Where("id = ?", id).Exec(ctx)
	return err
}

func (r *ModerationRepo) PurgeDomain(ctx context.Context, tx *dbPorts.Tx, domain string) (*models.PurgeDomainResult, []string, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, nil, err
	}
	domain = normalizeDomain(domain)
	patterns := domainActorPatterns(domain)

	var noteIDs []string
	if err := db.NewSelect().Model((*dbModels.Note)(nil)).Column("id").WhereGroup(" OR ", func(q *bun.SelectQuery) *bun.SelectQuery {
		for _, pattern := range patterns {
			q = q.WhereOr("attributed_to LIKE ?", pattern)
		}
		return q
	}).Scan(ctx, &noteIDs); err != nil {
		return nil, nil, err
	}

	var mediaRows []dbModels.MediaAttachment
	if len(noteIDs) > 0 {
		if err := db.NewSelect().Model(&mediaRows).
			Join("JOIN note_media_attachments AS nma ON nma.media_id = media_attachment.id").
			Where("nma.note_id IN (?)", bun.In(noteIDs)).
			Scan(ctx); err != nil {
			return nil, nil, err
		}
	}
	mediaIDs := make([]string, 0, len(mediaRows))
	storagePaths := make([]string, 0, len(mediaRows))
	for _, media := range mediaRows {
		mediaIDs = append(mediaIDs, media.ID)
		if media.StoragePath != "" {
			storagePaths = append(storagePaths, media.StoragePath)
		}
	}

	result := &models.PurgeDomainResult{Domain: domain}
	if len(noteIDs) > 0 {
		res, err := db.NewDelete().Model((*dbModels.Notification)(nil)).Where("status_id IN (?)", bun.In(noteIDs)).Exec(ctx)
		if err != nil {
			return nil, nil, err
		}
		result.DeletedNotifications += affected(res)
		if _, err := db.NewDelete().Model((*dbModels.Boost)(nil)).Where("note_id IN (?)", bun.In(noteIDs)).Exec(ctx); err != nil {
			return nil, nil, err
		}
		res, err = db.NewDelete().Model((*dbModels.Note)(nil)).Where("id IN (?)", bun.In(noteIDs)).Exec(ctx)
		if err != nil {
			return nil, nil, err
		}
		result.DeletedNotes = affected(res)
	}

	for _, pattern := range patterns {
		res, err := db.NewDelete().Model((*dbModels.Notification)(nil)).Where("actor_account_id LIKE ?", pattern).Exec(ctx)
		if err != nil {
			return nil, nil, err
		}
		result.DeletedNotifications += affected(res)
	}
	for _, pattern := range patterns {
		res, err := db.NewDelete().Model((*dbModels.Follow)(nil)).Where("remote_actor LIKE ?", pattern).Exec(ctx)
		if err != nil {
			return nil, nil, err
		}
		result.DeletedFollows += affected(res)
	}
	res, err := db.NewDelete().Model((*dbModels.RemoteAccount)(nil)).Where("domain = ?", domain).Exec(ctx)
	if err != nil {
		return nil, nil, err
	}
	result.DeletedRemoteAccounts = affected(res)
	if len(mediaIDs) > 0 {
		res, err := db.NewDelete().Model((*dbModels.MediaAttachment)(nil)).Where("id IN (?)", bun.In(mediaIDs)).Exec(ctx)
		if err != nil {
			return nil, nil, err
		}
		result.DeletedMedia = affected(res)
	}
	return result, storagePaths, nil
}

func normalizeDomain(domain string) string {
	return strings.Trim(strings.ToLower(domain), ". \t\n\r")
}

func domainActorPatterns(domain string) []string {
	return []string{"https://" + domain + "/%", "http://" + domain + "/%"}
}

func affected(res interface{ RowsAffected() (int64, error) }) int {
	count, err := res.RowsAffected()
	if err != nil {
		return 0
	}
	return int(count)
}
