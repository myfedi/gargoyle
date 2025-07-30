package password

import (
	"github.com/myfedi/gargoyle/domain/ports"
	"golang.org/x/crypto/bcrypt"
)

type BcryptPasswordHasher struct{}

func NewBCryptPasswordHasher() BcryptPasswordHasher {
	return BcryptPasswordHasher{}
}

// make sure we adhere to the port interface
var _ ports.PasswordHashProvider = &BcryptPasswordHasher{}

func (e BcryptPasswordHasher) HashPassword(password string) (string, error) {
	encryptedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return "", err
	}

	return string(encryptedPassword), nil
}

func (e BcryptPasswordHasher) CompareHashAndPassword(hash string, password string) error {
	return bcrypt.CompareHashAndPassword(
		[]byte(hash),
		[]byte(password),
	)
}
