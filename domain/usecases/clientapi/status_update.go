package clientapi

import (
	"context"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

type statusEdit struct {
	Original    models.Note
	Note        models.Note
	Visibility  string
	Media       []models.MediaAttachment
	Mentions    []models.Account
	PollOptions []string
	RawJSON     []byte
}

// UpdateStatus edits a local status and prepares the committed ActivityPub
// Update for delivery by the HTTP adapter after the transaction has closed.
func (u Statuses) UpdateStatus(ctx context.Context, account *models.Account, statusID string, input UpdateStatusInput) (*UpdateStatusResult, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}

	edit, derr := u.prepareStatusEdit(ctx, account, statusID, input)
	if derr != nil {
		return nil, derr
	}

	res, derr := u.persistStatusEdit(ctx, account, edit)
	if derr != nil {
		return nil, derr
	}

	return &UpdateStatusResult{
		Note:            res.Note,
		Account:         res.Account,
		Media:           edit.Media,
		Mentions:        res.Mentions,
		RawJSON:         res.RawJSON,
		FollowerInboxes: res.FollowerInboxes,
		MentionInboxes:  res.MentionInboxes,
	}, nil
}

func (u Statuses) prepareStatusEdit(ctx context.Context, account *models.Account, statusID string, input UpdateStatusInput) (*statusEdit, *domainerrors.DomainError) {
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
	if input.ObjectType == "" {
		input.ObjectType = note.ObjectType
	}
	if input.ObjectType == "Question" && len(input.PollOptions) == 0 {
		existing, err := u.deps.PollsRepo.GetPollOptions(ctx, nil, note.ID)
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		for _, option := range existing {
			input.PollOptions = append(input.PollOptions, option.Title)
		}
		input.PollMultiple = note.PollMultiple
	}
	pollOptions, pollExpiresAt, derr := pollInput(input.ObjectType, input.PollOptions, input.PollExpiresIn)
	if derr != nil {
		return nil, derr
	}
	if input.ObjectType == "Question" && input.PollExpiresIn == 0 {
		pollExpiresAt = note.PollExpiresAt
	}
	editedNote := statusWithEdits(*note, input, visibility, u.deps.ContentSanitizer.SanitizeHTML(input.Content), u.deps.ContentSanitizer.StripHTMLFromText(input.Content))
	editedAt := time.Now().UTC()
	editedNote.EditedAt = &editedAt
	if input.ObjectType == "Question" {
		editedNote.PollMultiple = input.PollMultiple
		editedNote.PollExpiresAt = pollExpiresAt
	}
	raw, derr := u.statusUpdateActivity(*account, editedNote, media, mentions, pollOptions)
	if derr != nil {
		return nil, derr
	}

	return &statusEdit{Original: *note, Note: editedNote, Visibility: visibility, Media: media, Mentions: mentions, PollOptions: pollOptions, RawJSON: raw}, nil
}

func (u Statuses) editableStatus(ctx context.Context, account *models.Account, statusID string) (*models.Note, *domainerrors.DomainError) {
	note, err := u.deps.NotesRepo.GetNoteByID(ctx, nil, statusID)
	if err != nil || note.LocalAccountID != account.ID || note.AttributedTo != account.URI {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "status not found")
	}
	return note, nil
}

func statusWithEdits(note models.Note, input UpdateStatusInput, visibility, content, plainText string) models.Note {
	note.Content = content
	note.PlainText = plainText
	note.Visibility = visibility
	note.Sensitive = input.Sensitive
	note.SpoilerText = input.SpoilerText
	note.ObjectType = apUsecases.NormalizeStatusObjectType(input.ObjectType)
	note.Hashtags = contentHashtags(input.Content)
	return note
}

func (u Statuses) persistStatusEdit(ctx context.Context, account *models.Account, edit *statusEdit) (*apUsecases.UpdateObjectResult, *domainerrors.DomainError) {
	res, derr := u.deps.UpdateObjectUC.UpdateObject(ctx, apUsecases.UpdateObjectInput{Username: account.Username, ObjectID: edit.Original.ID, UpdatedNote: edit.Note, RawJSON: edit.RawJSON, Media: edit.Media, Mentions: edit.Mentions, PollOptions: edit.PollOptions})
	if derr != nil {
		return nil, derr
	}
	return res, nil
}

func (u Statuses) statusUpdateActivity(account models.Account, note models.Note, media []models.MediaAttachment, mentions []models.Account, pollOptions []string) ([]byte, *domainerrors.DomainError) {
	activityID, err := u.deps.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return apUsecases.BuildStatusUpdateActivity(account, note, media, mentions, pollOptions, u.deps.Host, activityID)
}
