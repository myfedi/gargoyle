package repos

type PostsRepository interface {
	// Main usecase is the nodeinfo.
	GetLocalPostsCount() (int, error)
}
