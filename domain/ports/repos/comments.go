package repos

type CommentsRepository interface {
	// Main usecase is the nodeinfo.
	GetLocalCommentsCount() (int, error)
}
