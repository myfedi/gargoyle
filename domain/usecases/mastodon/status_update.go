package mastodon

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type statusEdit struct {
	Original   models.Note
	Note       models.Note
	Visibility string
	Media      []models.MediaAttachment
	Mentions   []models.Account
	RawJSON    []byte
}

// UpdateStatus edits a local status and prepares the committed ActivityPub
// Update for delivery by the HTTP adapter after the transaction has closed.
func (u UseCase) UpdateStatus(ctx context.Context, account *models.Account, statusID string, input UpdateStatusInput) (*UpdateStatusResult, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}

	edit, derr := u.prepareStatusEdit(ctx, account, statusID, input)
	if derr != nil {
		return nil, derr
	}

	updatedNote, derr := u.persistStatusEdit(ctx, account, edit)
	if derr != nil {
		return nil, derr
	}

	storedMentions, derr := u.storedStatusMentions(ctx, edit.Note.ID)
	if derr != nil {
		return nil, derr
	}
	followerInboxes, derr := u.followerInboxes(ctx, account.ID)
	if derr != nil {
		return nil, derr
	}

	return &UpdateStatusResult{
		Note:            *updatedNote,
		Account:         *account,
		Media:           edit.Media,
		Mentions:        storedMentions,
		RawJSON:         edit.RawJSON,
		FollowerInboxes: followerInboxes,
		MentionInboxes:  mentionInboxes(edit.Mentions),
	}, nil
}

func (u UseCase) prepareStatusEdit(ctx context.Context, account *models.Account, statusID string, input UpdateStatusInput) (*statusEdit, *domainerrors.DomainError) {
	if strings.TrimSpace(input.Content) == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "status is required")
	}

	note, derr := u.editableStatus(ctx, account, statusID)
	if derr != nil {
		return nil, derr
	}

	visibility := normalizedVisibility(input.Visibility)
	mentions, derr := u.resolveMentions(ctx, account, input.Content)
	if derr != nil {
		return nil, derr
	}
	if visibility == "direct" && len(mentions) == 0 {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "direct statuses require at least one mentioned recipient")
	}

	media, derr := u.statusMedia(ctx, account, input.MediaIDs)
	if derr != nil {
		return nil, derr
	}

	editedNote := statusWithEdits(*note, input, visibility, u.cfg.ContentSanitizer.SanitizeHTML(input.Content), u.cfg.ContentSanitizer.StripHTMLFromText(input.Content))
	raw, derr := u.statusUpdateActivity(*account, editedNote, media, mentions)
	if derr != nil {
		return nil, derr
	}

	return &statusEdit{Original: *note, Note: editedNote, Visibility: visibility, Media: media, Mentions: mentions, RawJSON: raw}, nil
}

func (u UseCase) editableStatus(ctx context.Context, account *models.Account, statusID string) (*models.Note, *domainerrors.DomainError) {
	note, err := u.cfg.NotesRepo.GetNoteByID(ctx, nil, statusID)
	if err != nil || note.LocalAccountID != account.ID || note.AttributedTo != account.URI {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "status not found")
	}
	return note, nil
}

func statusWithEdits(note models.Note, input UpdateStatusInput, visibility string, content string, plainText string) models.Note {
	note.Content = content
	note.PlainText = plainText
	note.Visibility = visibility
	note.Sensitive = input.Sensitive
	note.SpoilerText = input.SpoilerText
	return note
}

func (u UseCase) persistStatusEdit(ctx context.Context, account *models.Account, edit *statusEdit) (*models.Note, *domainerrors.DomainError) {
	var updated *models.Note
	err := u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		if err := u.storeCurrentStatusRevision(ctx, &tx, edit.Original.ID); err != nil {
			return err
		}
		stored, err := u.updateStatusNote(ctx, &tx, edit)
		if err != nil {
			return err
		}
		updated = stored
		if err := u.replaceStatusMedia(ctx, &tx, edit.Note.ID, edit.Media); err != nil {
			return err
		}
		if err := u.replaceStatusMentions(ctx, &tx, account.ID, edit.Note.ID, edit.Mentions); err != nil {
			return err
		}
		return u.storeStatusUpdateActivity(ctx, &tx, account, edit)
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return updated, nil
}

func (u UseCase) storeCurrentStatusRevision(ctx context.Context, tx *db.Tx, noteID string) error {
	note, err := u.cfg.NotesRepo.GetNoteByID(ctx, tx, noteID)
	if err != nil {
		return err
	}
	media, err := u.cfg.MediaRepo.ListMediaForNote(ctx, tx, noteID)
	if err != nil {
		return err
	}
	mediaIDs := make([]string, 0, len(media))
	for _, item := range media {
		mediaIDs = append(mediaIDs, item.ID)
	}
	createdAt := note.PublishedAt
	if note.EditedAt != nil {
		createdAt = *note.EditedAt
	}
	_, err = u.cfg.NotesRepo.CreateNoteEdit(ctx, tx, repos.CreateNoteEditInput{Note: *note, CreatedAt: createdAt, MediaIDs: mediaIDs})
	return err
}

func (u UseCase) updateStatusNote(ctx context.Context, tx *db.Tx, edit *statusEdit) (*models.Note, error) {
	return u.cfg.NotesRepo.UpdateNoteByID(ctx, tx, edit.Note.ID, repos.UpdateNoteInput{
		Content:     edit.Note.Content,
		PlainText:   edit.Note.PlainText,
		Visibility:  edit.Visibility,
		Sensitive:   edit.Note.Sensitive,
		SpoilerText: edit.Note.SpoilerText,
	})
}

func (u UseCase) replaceStatusMedia(ctx context.Context, tx *db.Tx, noteID string, media []models.MediaAttachment) error {
	mediaIDs := make([]string, 0, len(media))
	for _, item := range media {
		mediaIDs = append(mediaIDs, item.ID)
	}
	return u.cfg.MediaRepo.ReplaceMediaForNote(ctx, tx, noteID, mediaIDs)
}

func (u UseCase) replaceStatusMentions(ctx context.Context, tx *db.Tx, accountID string, noteID string, mentions []models.Account) error {
	if err := u.cfg.MentionsRepo.DeleteMentionsForNote(ctx, tx, noteID); err != nil {
		return err
	}
	for _, mention := range mentions {
		input := repos.CreateMentionInput{LocalAccountID: accountID, NoteID: noteID, AccountID: mention.ID, Username: mention.Username, Acct: mentionAcct(mention), URL: mentionURL(mention, u.cfg.Host)}
		if _, err := u.cfg.MentionsRepo.CreateMention(ctx, tx, input); err != nil {
			return err
		}
	}
	return nil
}

func (u UseCase) storeStatusUpdateActivity(ctx context.Context, tx *db.Tx, account *models.Account, edit *statusEdit) error {
	_, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, tx, repos.CreateActivityInput{
		LocalAccountID: account.ID,
		Direction:      models.ActivityDirectionOutbox,
		Type:           "Update",
		Actor:          account.URI,
		Object:         edit.Note.URI,
		RawJSON:        string(edit.RawJSON),
	})
	return err
}

func (u UseCase) storedStatusMentions(ctx context.Context, noteID string) ([]models.Mention, *domainerrors.DomainError) {
	mentions, err := u.cfg.MentionsRepo.ListMentionsForNote(ctx, nil, noteID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return mentions, nil
}

func (u UseCase) statusUpdateActivity(account models.Account, note models.Note, media []models.MediaAttachment, mentions []models.Account) ([]byte, *domainerrors.DomainError) {
	activityID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}

	noteDoc := statusUpdateNoteDocument(account, note)
	applyVisibilityAddressing(noteDoc, note.Visibility, &account, mentions)
	applyMediaAttachments(noteDoc, u.cfg.Host, media)

	raw, err := json.Marshal(map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       account.URI + "/updates/" + activityID,
		"type":     "Update",
		"actor":    account.URI,
		"to":       noteDoc["to"],
		"cc":       noteDoc["cc"],
		"object":   noteDoc,
	})
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return raw, nil
}

func statusUpdateNoteDocument(account models.Account, note models.Note) map[string]any {
	doc := map[string]any{
		"id":           note.URI,
		"type":         "Note",
		"content":      note.Content,
		"attributedTo": account.URI,
		"visibility":   note.Visibility,
		"sensitive":    note.Sensitive,
	}
	if !note.PublishedAt.IsZero() {
		doc["published"] = note.PublishedAt.UTC().Format(time.RFC3339)
	}
	if note.SpoilerText != "" {
		doc["summary"] = note.SpoilerText
	}
	if note.InReplyToURI != nil && *note.InReplyToURI != "" {
		doc["inReplyTo"] = *note.InReplyToURI
	}
	return doc
}
