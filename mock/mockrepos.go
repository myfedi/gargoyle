package mock

// TODO(christian): move this to testing later on, when we have proper adapters

import "github.com/myfedi/gargoyle/domain/ports/repos"

// == users repo

type MockUsersRepository struct {
	UsersCount int
}

func (m *MockUsersRepository) GetUsersCount() (int, error) {
	return m.UsersCount, nil
}

func (m *MockUsersRepository) UserWithUsernameExists(username string) (bool, error) {
	return username == "alice", nil
}

var _ repos.UsersRepository = &MockUsersRepository{}

// == posts repo

type MockPostsRepository struct {
	PostsCount int
}

func (m *MockPostsRepository) GetLocalPostsCount() (int, error) {
	return m.PostsCount, nil
}

var _ repos.PostsRepository = &MockPostsRepository{}

// == comments repo

type MockCommentsRepository struct {
	CommentsCount int
}

func (m *MockCommentsRepository) GetLocalCommentsCount() (int, error) {
	return m.CommentsCount, nil
}

var _ repos.CommentsRepository = &MockCommentsRepository{}
