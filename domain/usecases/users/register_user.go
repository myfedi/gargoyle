package users

import (
	"fmt"

	"github.com/myfedi/gargoyle/domain/models"
	errors "github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/gcrypto"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type RegisterUserUseCaseInput struct {
	Email    string
	Password string
	Username string
	Admin    bool
}

type RegisterUserUseCaseConfig struct {
	AccountsRepo         repos.AccountsRepo
	UsersRepo            repos.UsersRepository
	PasswordHashProvider ports.PasswordHashProvider
	PKeyManager          gcrypto.PKeyManager
	LocalDomain          string
}

type RegisterUserUseCase struct {
	cfg RegisterUserUseCaseConfig
}

func NewRegisterUserUseCase(cfg RegisterUserUseCaseConfig) RegisterUserUseCase {
	return RegisterUserUseCase{
		cfg: cfg,
	}
}

// RegisterUser creates a new user by hashing the password and persisting the user in the database.
// It is expected, that the input has been verified for format, length etc. beforehand.
func (u *RegisterUserUseCase) RegisterUser(input RegisterUserUseCaseInput) (*models.User, *errors.DomainError) {
	userNameTaken, err := u.cfg.UsersRepo.UserWithUsernameExists(input.Username)
	if err != nil {
		return nil, errors.NewErr(errors.ErrBadRequest, err)
	}
	if userNameTaken {
		return nil, errors.NewErr(errors.ErrBadRequest, fmt.Errorf("Username %s already taken", input.Username))
	}

	emailTaken, err := u.cfg.UsersRepo.UserWithEmailExists(input.Email)
	if err != nil {
		return nil, errors.NewErr(errors.ErrBadRequest, err)
	}
	if emailTaken {
		return nil, errors.NewErr(errors.ErrBadRequest, fmt.Errorf("Email %s already taken", input.Email))
	}

	hashedPass, err := u.cfg.PasswordHashProvider.HashPassword(input.Password)
	if err != nil {
		return nil, errors.NewErr(errors.ErrInternal, err)
	}

	user, err := u.cfg.UsersRepo.CreateUser(repos.UserCreationInput{
		HashedPassword: hashedPass,
		Email:          input.Email,
		Admin:          input.Admin,
	})
	if err != nil {
		// TODO: do we need better error resolution here?
		return nil, errors.NewErr(errors.ErrInternal, err)
	}

	// create pkey
	pkey, err := u.cfg.PKeyManager.CreatePKeyPair(input.Email)
	if err != nil {
		return nil, errors.NewErr(errors.ErrInternal, err)
	}

	publicPem := string(pkey.PublicKey().ToPEM())
	privatePem := string(pkey.PrivateKey().ToPEM())

	// create account
	_, err = u.cfg.AccountsRepo.CreateAccount(repos.CreateAccountInput{
		UserID:      user.ID,
		Username:    input.Username,
		Domain:      u.cfg.LocalDomain,
		DisplayName: input.Username,
		PrivateKey:  &privatePem,
		PublicKey:   publicPem,
		ActorType:   models.ActorTypePerson,
	})
	if err != nil {
		// TODO: do we need better error resolution here?
		return nil, errors.NewErr(errors.ErrInternal, err)
	}

	return user, nil
}
