package repos

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	"github.com/myfedi/gargoyle/infrastructure/db"
	dbModels "github.com/myfedi/gargoyle/infrastructure/db/models"
	"github.com/uptrace/bun"
)

type AccountsRepo struct {
	db *bun.DB
}

func NewAccountsRepo(db *bun.DB) *AccountsRepo {
	return &AccountsRepo{db: db}
}

func (r *AccountsRepo) CreateAccount(input repos.CreateAccountInput) (*models.Account, error) {
	ulid, err := db.NewULID()
	if err != nil {
		return nil, err
	}

	account := &models.Account{
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
		InboxURI:              input.InboxURI,
		OutboxURI:             input.OutboxURI,
		FollowingURI:          input.FollowingURI,
		FollowersURI:          input.FollowersURI,
		FeaturedCollectionURI: input.FeaturedCollectionURI,
		PrivateKey:            input.PrivateKey,
		PublicKey:             input.PublicKey,
	}

	_, err = r.db.NewInsert().Model(account).Exec(context.Background())
	if err != nil {
		return nil, err
	}

	return account, nil
}

func (r *AccountsRepo) GetAccountByUserID(userID string) (*models.Account, error) {
	var account dbModels.Account
	err := r.db.NewSelect().
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
