package repos

import "context"

type CommentsRepository interface {
	// Main usecase is the nodeinfo.
	GetLocalCommentsCount(ctx context.Context) (int, error)
}
