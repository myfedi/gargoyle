package mock

// TODO(christian): move this to testing later on, when we have proper adapters

import (
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

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
