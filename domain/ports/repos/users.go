package repos

type UsersRepository interface {
	GetUsersCount() (int, error)
	UserWithUsernameExists(username string) (bool, error)
}
