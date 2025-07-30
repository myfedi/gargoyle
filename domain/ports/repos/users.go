package repos

import (
	"github.com/myfedi/gargoyle/domain/models"
	errors "github.com/myfedi/gargoyle/domain/models/domainerrors"
)

type UserCreationInput struct {
	HashedPassword string
	Email          string
	Admin          bool
}
type UsersRepository interface {
	GetUsersCount() (int, error)
	UserWithUsernameExists(username string) (bool, error)
	UserWithEmailExists(email string) (bool, error)
	GetUserByUsername(username string) (*models.Account, *errors.DomainError)
	CreateUser(input UserCreationInput) (*models.User, error)
}
