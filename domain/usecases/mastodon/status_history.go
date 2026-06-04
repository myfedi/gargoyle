package mastodon

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

type StatusEditHistoryItem struct {
	Content     string
	PlainText   string
	ObjectType  string
	SpoilerText string
	Sensitive   bool
	CreatedAt   time.Time
	Account     models.Account
	Media       []models.MediaAttachment
}

func (u UseCase) StatusHistory(ctx context.Context, localAccount *models.Account, statusID string) ([]StatusEditHistoryItem, *domainerrors.DomainError) {
	item, derr := u.GetStatus(ctx, localAccount, statusID)
	if derr != nil {
		return nil, derr
	}
	edits, err := u.cfg.NotesRepo.ListNoteEdits(ctx, nil, item.Note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res := make([]StatusEditHistoryItem, 0, len(edits)+1)
	for _, edit := range edits {
		media, derr := u.mediaForHistoryEdit(ctx, edit.MediaIDs)
		if derr != nil {
			return nil, derr
		}
		res = append(res, StatusEditHistoryItem{Content: edit.Content, PlainText: edit.PlainText, ObjectType: edit.ObjectType, SpoilerText: edit.SpoilerText, Sensitive: edit.Sensitive, CreatedAt: historyCreatedAt(edit.CreatedAt, item.Note), Account: item.Account, Media: media})
	}
	created := item.Note.PublishedAt
	if item.Note.EditedAt != nil {
		created = *item.Note.EditedAt
	}
	res = append(res, StatusEditHistoryItem{Content: item.Note.Content, PlainText: item.Note.PlainText, ObjectType: item.Note.ObjectType, SpoilerText: item.Note.SpoilerText, Sensitive: item.Note.Sensitive, CreatedAt: historyCreatedAt(created, item.Note), Account: item.Account, Media: item.Media})
	return res, nil
}

func (u UseCase) mediaForHistoryEdit(ctx context.Context, mediaIDs []string) ([]models.MediaAttachment, *domainerrors.DomainError) {
	media := make([]models.MediaAttachment, 0, len(mediaIDs))
	for _, id := range mediaIDs {
		item, err := u.cfg.MediaRepo.GetMediaAttachmentByID(ctx, nil, id)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		media = append(media, *item)
	}
	return media, nil
}

func historyCreatedAt(created time.Time, note models.Note) time.Time {
	if !created.IsZero() {
		return created
	}
	if !note.PublishedAt.IsZero() {
		return note.PublishedAt
	}
	if !note.CreatedAt.IsZero() {
		return note.CreatedAt
	}
	return time.Now().UTC()
}
