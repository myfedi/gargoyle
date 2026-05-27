package repos

import "context"

type PostsRepository interface {
	// Main usecase is the nodeinfo.
	GetLocalPostsCount(ctx context.Context) (int, error)
}
