package repos

import (
	"context"
	"errors"

	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	dbPorts "github.com/myfedi/gargoyle/domain/ports/db"
	ports "github.com/myfedi/gargoyle/domain/ports/repos"
	dbUtils "github.com/myfedi/gargoyle/infrastructure/db"
	dbModels "github.com/myfedi/gargoyle/infrastructure/db/models"
	"github.com/uptrace/bun"
)

type UsersRepo struct {
	db bun.IDB
}

func NewUsersRepo(db *bun.DB) UsersRepo {
	return UsersRepo{
		db: db,
	}
}

// check that adapter implements port
var _ ports.UsersRepository = &UsersRepo{}

func (r UsersRepo) GetUsersCount(tx *dbPorts.Tx) (int, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return 0, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	return db.NewSelect().Model((*models.User)(nil)).Count(context.Background())
}

func (r UsersRepo) UserWithUsernameExists(tx *dbPorts.Tx, username string) (bool, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return false, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	return db.NewSelect().
		Model((*dbModels.User)(nil)).
		Where("username = ?", username).
		Exists(context.Background())
}

func (r UsersRepo) UserWithEmailExists(tx *dbPorts.Tx, email string) (bool, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return false, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	return db.NewSelect().
		Model((*models.User)(nil)).
		Where("email = ?", email).
		Exists(context.Background())
}

func (r UsersRepo) GetUserByUsername(tx *dbPorts.Tx, username string) (*models.Account, *domainerrors.DomainError) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return nil, domainerrors.New(domainerrors.ErrInternal, "unexpected tx implementation provided")
		}
	}

	var account dbModels.Account
	err := db.NewSelect().
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

func (r UsersRepo) CreateUser(tx *dbPorts.Tx, input ports.UserCreationInput) (*models.User, error) {
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

	user := &dbModels.User{
		ID:           ulid,
		Username:     input.Username,
		PasswordHash: input.HashedPassword,
		Email:        input.Email,
		Admin:        input.Admin,
	}

	_, err = db.NewInsert().Model(user).Exec(context.Background())
	if err != nil {
		return nil, err
	}

	model := user.ToModel()

	return &model, nil
}
