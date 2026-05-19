package repos

import (
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
	GetUsersCount(tx *db.Tx) (int, error)
	UserWithUsernameExists(tx *db.Tx, username string) (bool, error)
	UserWithEmailExists(tx *db.Tx, email string) (bool, error)
	CreateUser(tx *db.Tx, input UserCreationInput) (*models.User, error)
}
