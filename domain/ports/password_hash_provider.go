package ports

type PasswordHashProvider interface {
	HashPassword(password string) (string, error)
	CompareHashAndPassword(hash, password string) error
}
