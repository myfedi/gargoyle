package repos

import (
	"context"
	"errors"
	"time"

	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	"github.com/myfedi/gargoyle/domain/models"
	dbPorts "github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	dbModels "github.com/myfedi/gargoyle/infrastructure/db/models"
	"github.com/uptrace/bun"
)

type RemoteAccountsRepo struct{ db bun.IDB }

func NewRemoteAccountsRepo(db *bun.DB) *RemoteAccountsRepo { return &RemoteAccountsRepo{db: db} }

var _ repos.RemoteAccountsRepository = &RemoteAccountsRepo{}

func (r *RemoteAccountsRepo) resolveDB(tx *dbPorts.Tx) (bun.IDB, error) {
	if tx == nil {
		return r.db, nil
	}
	adapted, ok := (*tx).(dbAdapters.BunTx)
	if !ok {
		return nil, errors.New(unexpectedTxImplementationError)
	}
	return adapted.Unwrap(), nil
}

func (r *RemoteAccountsRepo) UpsertRemoteAccount(ctx context.Context, tx *dbPorts.Tx, account models.Account) (*models.Account, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	row := dbModels.RemoteAccountFromModel(account)
	now := time.Now().UTC()
	row.FetchedAt = now
	_, err = db.NewInsert().Model(&row).
		On("CONFLICT (uri) DO UPDATE").
		Set("updated_at = ?", now).
		Set("fetched_at = ?", now).
		Set("username = EXCLUDED.username").
		Set("domain = EXCLUDED.domain").
		Set("display_name = EXCLUDED.display_name").
		Set("summary = EXCLUDED.summary").
		Set("url = EXCLUDED.url").
		Set("profile_fields = EXCLUDED.profile_fields").
		Set("avatar_media_id = COALESCE(EXCLUDED.avatar_media_id, avatar_media_id)").
		Set("header_media_id = COALESCE(EXCLUDED.header_media_id, header_media_id)").
		Set("avatar_url = EXCLUDED.avatar_url").
		Set("header_url = EXCLUDED.header_url").
		Set("inbox_uri = EXCLUDED.inbox_uri").
		Set("outbox_uri = EXCLUDED.outbox_uri").
		Set("following_uri = EXCLUDED.following_uri").
		Set("followers_uri = EXCLUDED.followers_uri").
		Set("featured_collection_uri = EXCLUDED.featured_collection_uri").
		Set("public_key = EXCLUDED.public_key").
		Set("actor_type = EXCLUDED.actor_type").
		Set("locked = EXCLUDED.locked").
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetRemoteAccountByURI(ctx, tx, account.URI)
}

func (r *RemoteAccountsRepo) SearchRemoteAccounts(ctx context.Context, tx *dbPorts.Tx, query string, limit int) ([]models.Account, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	pattern := "%" + query + "%"
	var rows []dbModels.RemoteAccount
	err = db.NewSelect().Model(&rows).
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("username LIKE ?", pattern).WhereOr("domain LIKE ?", pattern).WhereOr("display_name LIKE ?", pattern).WhereOr("uri LIKE ?", pattern).WhereOr("url LIKE ?", pattern).WhereOr("(username || '@' || COALESCE(domain, '')) LIKE ?", pattern)
		}).
		Order("fetched_at DESC").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	res := make([]models.Account, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.ToModel())
	}
	return res, nil
}

func (r *RemoteAccountsRepo) GetRemoteAccountByURI(ctx context.Context, tx *dbPorts.Tx, uri string) (*models.Account, error) {
	db, err := r.resolveDB(tx)
	if err != nil {
		return nil, err
	}
	var row dbModels.RemoteAccount
	if err := db.NewSelect().Model(&row).Where("uri = ?", uri).Scan(ctx); err != nil {
		return nil, err
	}
	model := row.ToModel()
	return &model, nil
}
