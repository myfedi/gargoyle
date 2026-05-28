package repos

import (
	"context"
	"errors"

	dbAdapters "github.com/myfedi/gargoyle/adapters/db"
	"github.com/myfedi/gargoyle/domain/models"
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

func (r UsersRepo) GetUsersCount(ctx context.Context, tx *dbPorts.Tx) (int, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return 0, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	return db.NewSelect().Model((*models.User)(nil)).Count(ctx)
}

func (r UsersRepo) UserWithUsernameExists(ctx context.Context, tx *dbPorts.Tx, username string) (bool, error) {
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
		Exists(ctx)
}

func (r UsersRepo) UserWithEmailExists(ctx context.Context, tx *dbPorts.Tx, email string) (bool, error) {
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
		Exists(ctx)
}

func (r UsersRepo) GetUserByUsername(ctx context.Context, tx *dbPorts.Tx, username string) (*models.User, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	var user dbModels.User
	if err := db.NewSelect().Model(&user).Where("username = ?", username).Scan(ctx); err != nil {
		return nil, err
	}
	model := user.ToModel()
	return &model, nil
}

func (r UsersRepo) GetUserByEmail(ctx context.Context, tx *dbPorts.Tx, email string) (*models.User, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	var user dbModels.User
	if err := db.NewSelect().Model(&user).Where("email = ?", email).Scan(ctx); err != nil {
		return nil, err
	}
	model := user.ToModel()
	return &model, nil
}

func (r UsersRepo) GetUserByID(ctx context.Context, tx *dbPorts.Tx, id string) (*models.User, error) {
	db := r.db
	if tx != nil {
		if adapted, ok := (*tx).(dbAdapters.BunTx); ok {
			db = adapted.Unwrap()
		} else {
			return nil, errors.New("internal error: unexpected tx implementation provided")
		}
	}

	var user dbModels.User
	if err := db.NewSelect().Model(&user).Where("id = ?", id).Scan(ctx); err != nil {
		return nil, err
	}
	model := user.ToModel()
	return &model, nil
}

func (r UsersRepo) CreateUser(ctx context.Context, tx *dbPorts.Tx, input ports.UserCreationInput) (*models.User, error) {
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

	_, err = db.NewInsert().Model(user).Exec(ctx)
	if err != nil {
		return nil, err
	}

	model := user.ToModel()

	return &model, nil
}
