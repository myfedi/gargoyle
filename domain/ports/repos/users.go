package repos

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

type UserCreationInput struct {
	HashedPassword string
	Username       string
	Email          string
	Admin          bool
}
type UsersRepository interface {
	GetUsersCount(ctx context.Context, tx *db.Tx) (int, error)
	UserWithUsernameExists(ctx context.Context, tx *db.Tx, username string) (bool, error)
	UserWithEmailExists(ctx context.Context, tx *db.Tx, email string) (bool, error)
	CreateUser(ctx context.Context, tx *db.Tx, input UserCreationInput) (*models.User, error)
}
