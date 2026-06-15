package activitypub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
)

// DereferenceResult is a JSON-LD document that can be returned for a local
// ActivityPub ID. It is intentionally raw JSON because these routes serve
// protocol documents, not application DTOs.
type DereferenceResult struct {
	JSON []byte
}

func (u GetDereferenceUseCase) GetObject(ctx context.Context, username, objectID string) (*DereferenceResult, *domainerrors.DomainError) {
	return u.getObject(ctx, username, objectID, "")
}

func (u GetDereferenceUseCase) GetObjectForRequester(ctx context.Context, username, objectID, requesterActor string) (*DereferenceResult, *domainerrors.DomainError) {
	return u.getObject(ctx, username, objectID, requesterActor)
}

func (u GetDereferenceUseCase) getObject(ctx context.Context, username, objectID, requesterActor string) (*DereferenceResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, username)
	if derr != nil {
		return nil, derr
	}
	if objectID == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "missing object id")
	}
	note, derr := u.localDereferenceableNoteByURI(ctx, account, account.URI+"/objects/"+url.PathEscape(objectID), requesterActor)
	if derr != nil {
		return nil, derr
	}
	media, derr := u.noteMedia(ctx, note.ID)
	if derr != nil {
		return nil, derr
	}
	pollOptions, derr := u.pollOptions(ctx, note)
	if derr != nil {
		return nil, derr
	}
	raw, err := MarshalNoteObject(*note, *account, media, pollOptions, u.cfg.Host)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &DereferenceResult{JSON: raw}, nil
}

func (u GetDereferenceUseCase) GetActivity(ctx context.Context, username, activityID string) (*DereferenceResult, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, username)
	if derr != nil {
		return nil, derr
	}
	if activityID == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "missing activity id")
	}
	activityURI := account.URI + activityPathSegment + url.PathEscape(activityID)
	activity, err := u.cfg.ActivitiesRepo.GetOutboxActivityByURI(ctx, nil, account.ID, activityURI)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainerrors.New(domainerrors.ErrNotFound, "activity not found")
		}
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if activity.Object == "" {
		return nil, domainerrors.New(domainerrors.ErrNotFound, "activity not found")
	}
	if _, derr := u.localPublicNoteByURI(ctx, account, activity.Object); derr != nil {
		return nil, derr
	}
	return &DereferenceResult{JSON: []byte(activity.RawJSON)}, nil
}

func (u GetDereferenceUseCase) localPublicNoteByURI(ctx context.Context, account *models.Account, uri string) (*models.Note, *domainerrors.DomainError) {
	return u.localDereferenceableNoteByURI(ctx, account, uri, "")
}

func (u GetDereferenceUseCase) localDereferenceableNoteByURI(ctx context.Context, account *models.Account, uri, requesterActor string) (*models.Note, *domainerrors.DomainError) {
	note, err := u.cfg.NotesRepo.GetNoteByURI(ctx, nil, uri)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainerrors.New(domainerrors.ErrNotFound, objectNotFoundMessage)
		}
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if note.LocalAccountID != account.ID || note.AttributedTo != account.URI {
		return nil, domainerrors.New(domainerrors.ErrNotFound, objectNotFoundMessage)
	}
	if publiclyDereferenceable(note.Visibility) {
		return note, nil
	}
	if note.Visibility == "private" && requesterActor != "" && u.cfg.FollowsRepo != nil {
		follow, err := u.cfg.FollowsRepo.GetFollowByActor(ctx, nil, account.ID, requesterActor, "follower")
		if err == nil && follow.AcceptedAt != nil {
			return note, nil
		}
	}
	return nil, domainerrors.New(domainerrors.ErrNotFound, objectNotFoundMessage)
}

func (u GetDereferenceUseCase) pollOptions(ctx context.Context, note *models.Note) ([]string, *domainerrors.DomainError) {
	if u.cfg.PollsRepo == nil || note.ObjectType != "Question" {
		return nil, nil
	}
	options, err := u.cfg.PollsRepo.GetPollOptions(ctx, nil, note.ID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res := make([]string, 0, len(options))
	for _, option := range options {
		res = append(res, option.Title)
	}
	return res, nil
}

func (u GetDereferenceUseCase) noteMedia(ctx context.Context, noteID string) ([]models.MediaAttachment, *domainerrors.DomainError) {
	if u.cfg.MediaRepo == nil {
		return nil, nil
	}
	media, err := u.cfg.MediaRepo.ListMediaForNote(ctx, nil, noteID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return media, nil
}

func publiclyDereferenceable(visibility string) bool {
	switch visibility {
	case "", "public", "unlisted":
		return true
	default:
		return false
	}
}

// MarshalNoteObject serializes a local Note for direct ActivityPub object
// dereferencing. It uses the persisted note state so edits are reflected, while
// still preserving ActivityStreams-compatible addressing and attachments.
func MarshalNoteObject(note models.Note, account models.Account, media []models.MediaAttachment, pollOptions []string, host string) ([]byte, error) {
	object := noteObjectDocument(note, account)
	applyPollOptions(object, note, pollOptions)
	object[activityStreamsContextKey] = activityStreamsContextURI
	applyNoteMediaAttachments(object, host, media)
	return json.Marshal(object)
}

func noteObjectDocument(note models.Note, account models.Account) map[string]any {
	to := []string{activityStreamsPublicURI}
	cc := []string{account.FollowersURI}
	if note.Visibility == "private" {
		to = []string{account.FollowersURI}
		cc = []string{}
	} else if note.Visibility == "direct" {
		to = []string{}
		cc = []string{}
	}
	published := note.PublishedAt
	if published.IsZero() {
		published = note.CreatedAt
	}
	object := map[string]any{
		"id":           note.URI,
		"type":         noteDocumentType(note.ObjectType),
		"attributedTo": account.URI,
		"content":      note.Content,
		"published":    published.UTC().Format(time.RFC3339),
		"to":           to,
		"cc":           cc,
	}
	if note.SpoilerText != "" {
		object["summary"] = note.SpoilerText
	}
	if note.Sensitive {
		object["sensitive"] = true
	}
	if note.InReplyToURI != nil && *note.InReplyToURI != "" {
		object["inReplyTo"] = *note.InReplyToURI
	}
	if note.EditedAt != nil {
		object["updated"] = note.EditedAt.UTC().Format(time.RFC3339)
	}
	return object
}

func applyPollOptions(noteDoc map[string]any, note models.Note, options []string) {
	if noteDocumentType(note.ObjectType) != "Question" || len(options) == 0 {
		return
	}
	items := make([]map[string]string, 0, len(options))
	for _, option := range options {
		items = append(items, map[string]string{"type": "Note", "name": option})
	}
	if note.PollMultiple {
		noteDoc["anyOf"] = items
	} else {
		noteDoc["oneOf"] = items
	}
	if note.PollExpiresAt != nil {
		noteDoc["endTime"] = note.PollExpiresAt.UTC().Format(time.RFC3339)
	}
}

func noteDocumentType(objectType string) string {
	switch objectType {
	case "Article", "Page", "Question":
		return objectType
	default:
		return "Note"
	}
}

func applyNoteMediaAttachments(noteDoc map[string]any, host string, media []models.MediaAttachment) {
	if len(media) == 0 || host == "" {
		return
	}
	attachments := make([]map[string]string, 0, len(media))
	base := strings.TrimRight(host, "/")
	for _, item := range media {
		attachment := map[string]string{"type": "Document", "mediaType": item.ContentType, "url": base + "/media/" + item.ID + "/" + url.PathEscape(item.FileName)}
		if item.Description != "" {
			attachment["name"] = item.Description
		}
		attachments = append(attachments, attachment)
	}
	noteDoc["attachment"] = attachments
}
