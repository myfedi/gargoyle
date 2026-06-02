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

type AccountsRepo struct {
	db bun.IDB
}

func NewAccountsRepo(db *bun.DB) *AccountsRepo {
	return &AccountsRepo{db: db}
}

func (r *AccountsRepo) CreateAccount(ctx context.Context, tx *dbPorts.Tx, input repos.CreateAccountInput) (*models.Account, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	ulid, err := dbUtils.NewULID()
	if err != nil {
		return nil, err
	}

	account := &dbModels.Account{
		ID:                    ulid,
		UserID:                input.UserID,
		CreatedAt:             input.CreatedAt,
		UpdatedAt:             input.UpdatedAt,
		FetchedAt:             input.FetchedAt,
		Username:              input.Username,
		Domain:                input.Domain,
		DisplayName:           input.DisplayName,
		Summary:               input.Summary,
		URI:                   input.URI,
		URL:                   input.URL,
		AvatarMediaID:         input.AvatarMediaID,
		HeaderMediaID:         input.HeaderMediaID,
		AvatarURL:             input.AvatarURL,
		HeaderURL:             input.HeaderURL,
		InboxURI:              input.InboxURI,
		OutboxURI:             input.OutboxURI,
		FollowingURI:          input.FollowingURI,
		FollowersURI:          input.FollowersURI,
		FeaturedCollectionURI: input.FeaturedCollectionURI,
		PrivateKey:            input.PrivateKey,
		PublicKey:             input.PublicKey,
		ActorType:             int(input.ActorType),
	}

	_, err = db.NewInsert().Model(account).Exec(ctx)
	if err != nil {
		return nil, err
	}

	model := account.ToModel()
	return &model, nil
}

func (r *AccountsRepo) GetAccountByID(ctx context.Context, tx *dbPorts.Tx, id string) (*models.Account, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	var account dbModels.Account
	if err := db.NewSelect().Model(&account).Where("id = ?", id).Scan(ctx); err != nil {
		return nil, err
	}
	model := account.ToModel()
	return &model, nil
}

func (r *AccountsRepo) UpdateLocalAccountProfile(ctx context.Context, tx *dbPorts.Tx, id string, input repos.UpdateAccountProfileInput) (*models.Account, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	updatedAt := input.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err := db.NewUpdate().
		Model((*dbModels.Account)(nil)).
		Set("display_name = ?", input.DisplayName).
		Set("summary = ?", input.Summary).
		Set("avatar_media_id = ?", input.AvatarMediaID).
		Set("header_media_id = ?", input.HeaderMediaID).
		Set("avatar_url = ?", input.AvatarURL).
		Set("header_url = ?", input.HeaderURL).
		Set("updated_at = ?", updatedAt).
		Where("id = ?", id).
		Where("user_id IS NOT NULL").
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetAccountByID(ctx, tx, id)
}

func (r *AccountsRepo) GetAccountByUserID(ctx context.Context, tx *dbPorts.Tx, userID string) (*models.Account, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	var account dbModels.Account
	err := db.NewSelect().
		Model(&account).
		Where("user_id = ?", userID).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	model := account.ToModel()
	return &model, nil
}

func (r *AccountsRepo) GetLocalAccountByUsername(ctx context.Context, tx *dbPorts.Tx, username string) (*models.Account, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	var account dbModels.Account
	err := db.NewSelect().
		Model(&account).
		Where("username = ?", username).
		Where("user_id IS NOT NULL").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	model := account.ToModel()
	return &model, nil
}

func (r *AccountsRepo) SearchLocalAccounts(ctx context.Context, tx *dbPorts.Tx, query string, limit int) ([]models.Account, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
	}
	if limit <= 0 || limit > 40 {
		limit = 20
	}
	pattern := "%" + query + "%"
	var rows []dbModels.Account
	err := db.NewSelect().Model(&rows).
		Where("user_id IS NOT NULL").
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("username LIKE ?", pattern).WhereOr("display_name LIKE ?", pattern)
		}).
		Order("username ASC").
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

func (r AccountsRepo) AccountWithUsernameExists(ctx context.Context, tx *dbPorts.Tx, username string) (bool, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return false, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	return db.NewSelect().
		Model((*dbModels.Account)(nil)).
		Where("username = ?", username).
		Exists(ctx)
}
