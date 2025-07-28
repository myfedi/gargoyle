package ports

type PasswordHashProvider interface {
	HashPassword(password string) (string, error)
	CompareHashAndPassword(hash string, password string) error
}
