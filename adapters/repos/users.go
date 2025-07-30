package repos

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	ports "github.com/myfedi/gargoyle/domain/ports/repos"
	"github.com/myfedi/gargoyle/infrastructure/db"
	dbModels "github.com/myfedi/gargoyle/infrastructure/db/models"
	"github.com/uptrace/bun"
)

type UsersRepo struct {
	db *bun.DB
}

func NewUsersRepo(db *bun.DB) UsersRepo {
	return UsersRepo{
		db: db,
	}
}

// check that adapter implements port
var _ ports.UsersRepository = &UsersRepo{}

func (r UsersRepo) GetUsersCount() (int, error) {
	return r.db.NewSelect().Model((*models.User)(nil)).Count(context.Background())
}

func (r UsersRepo) UserWithUsernameExists(username string) (bool, error) {
	return r.db.NewSelect().
		Model((*dbModels.Account)(nil)).
		Where("username = ?", username).
		Exists(context.Background())
}

func (r UsersRepo) UserWithEmailExists(email string) (bool, error) {
	return r.db.NewSelect().
		Model((*models.User)(nil)).
		Where("email = ?", email).
		Exists(context.Background())
}

func (r UsersRepo) GetUserByUsername(username string) (*models.Account, *domainerrors.DomainError) {
	var account dbModels.Account
	err := r.db.NewSelect().
		Model(&account).
		Where("username = ?", username).
		Relation("User").
		Limit(1).
		Scan(context.Background())

	if err != nil {
		return nil, domainerrors.New(domainerrors.ErrInternal, "failed to query user")
	}

	model := account.ToModel()

	return &model, nil
}

func (r UsersRepo) CreateUser(input ports.UserCreationInput) (*models.User, error) {
	ulid, err := db.NewULID()
	if err != nil {
		return nil, err
	}

	user := &dbModels.User{
		ID:           ulid,
		PasswordHash: input.HashedPassword,
		Email:        input.Email,
		Admin:        input.Admin,
	}

	_, err = r.db.NewInsert().Model(user).Exec(context.Background())
	if err != nil {
		return nil, err
	}

	model := user.ToModel()

	return &model, nil
}
