package repos

import (
	"context"
	"errors"

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

func (r *AccountsRepo) CreateAccount(tx *dbPorts.Tx, input repos.CreateAccountInput) (*models.Account, error) {
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
		InboxURI:              input.InboxURI,
		OutboxURI:             input.OutboxURI,
		FollowingURI:          input.FollowingURI,
		FollowersURI:          input.FollowersURI,
		FeaturedCollectionURI: input.FeaturedCollectionURI,
		PrivateKey:            input.PrivateKey,
		PublicKey:             input.PublicKey,
		ActorType:             int(input.ActorType),
	}

	_, err = db.NewInsert().Model(account).Exec(context.Background())
	if err != nil {
		return nil, err
	}

	model := account.ToModel()
	return &model, nil
}

func (r *AccountsRepo) GetAccountByUserID(tx *dbPorts.Tx, userID string) (*models.Account, error) {
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
		Scan(context.Background())
	if err != nil {
		return nil, err
	}
	model := account.ToModel()
	return &model, nil
}

func (r *AccountsRepo) GetLocalAccountByUsername(tx *dbPorts.Tx, username string) (*models.Account, error) {
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
		Scan(context.Background())
	if err != nil {
		return nil, err
	}
	model := account.ToModel()
	return &model, nil
}

func (r AccountsRepo) AccountWithUsernameExists(tx *dbPorts.Tx, username string) (bool, error) {
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
		Exists(context.Background())
}
