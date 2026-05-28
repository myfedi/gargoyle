package mastodon

import (
	"context"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

// HomeTimeline returns notes addressed to the authenticated account. Each item
// carries the account that authored the note so Mastodon responses can render
// remote statuses as remote authors instead of as the local timeline owner.
func (u UseCase) HomeTimeline(ctx context.Context, account *models.Account) ([]TimelineItem, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	notes, err := u.cfg.NotesRepo.ListLocalNotes(ctx, nil, account.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	items := make([]TimelineItem, 0, len(notes))
	for _, note := range notes {
		author, derr := u.noteAuthor(ctx, account, note)
		if derr != nil {
			return nil, derr
		}
		items = append(items, TimelineItem{Note: note, Account: *author})
	}
	return items, nil
}

func (u UseCase) PublicTimeline(ctx context.Context, account *models.Account) ([]TimelineItem, *domainerrors.DomainError) {
	return u.HomeTimeline(ctx, account)
}

func (u UseCase) noteAuthor(ctx context.Context, localAccount *models.Account, note models.Note) (*models.Account, *domainerrors.DomainError) {
	if note.AttributedTo == "" || note.AttributedTo == localAccount.URI {
		return localAccount, nil
	}
	remote, err := u.cfg.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, nil, note.AttributedTo)
	if err == nil {
		return remote, nil
	}
	remote, err = u.resolveAndCacheRemoteAccount(ctx, note.AttributedTo, localAccount)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return remote, nil
}
