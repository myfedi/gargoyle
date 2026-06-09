package clientapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/repos"
	apUsecases "github.com/myfedi/gargoyle/domain/usecases/activitypub"
)

const (
	maxProfileDisplayNameLength = 120
	maxProfileNoteLength        = 5000
	maxProfileFields            = 4
	maxProfileFieldNameLength   = 255
	maxProfileFieldValueLength  = 2047
)

func (u Profile) UpdateCredentials(ctx context.Context, account *models.Account, input UpdateCredentialsInput) (*UpdateCredentialsResult, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	if account.UserID == nil {
		return nil, domainerrors.New(domainerrors.ErrUnauthorized, "only local accounts can update credentials")
	}

	displayName, sanitizedNote, fields, derr := u.validatedProfileText(input)
	if derr != nil {
		return nil, derr
	}
	avatarMediaID, headerMediaID, createdMedia, derr := u.profileMediaIDs(ctx, account, input)
	if derr != nil {
		return nil, derr
	}

	updateID, err := u.deps.IDGenerator.NewID()
	if err != nil {
		u.deleteCreatedProfileMedia(ctx, createdMedia)
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	res, derr := u.deps.UpdateActorUC.UpdateActor(ctx, apUsecases.UpdateActorInput{Username: account.Username, UpdateID: updateID, DisplayName: stringPtrOrNil(displayName), Summary: stringPtrOrNil(sanitizedNote), Fields: fields, AvatarMediaID: avatarMediaID, HeaderMediaID: headerMediaID, Locked: &input.Locked})
	if derr != nil {
		u.deleteCreatedProfileMedia(ctx, createdMedia)
		return nil, derr
	}
	return &UpdateCredentialsResult{Account: res.Account, RawJSON: res.RawJSON, FollowerInboxes: res.FollowerInboxes}, nil
}

func (u Profile) validatedProfileText(input UpdateCredentialsInput) (string, string, []models.AccountProfileField, *domainerrors.DomainError) {
	displayName := strings.TrimSpace(input.DisplayName)
	if len([]rune(displayName)) > maxProfileDisplayNameLength {
		return "", "", nil, domainerrors.New(domainerrors.ErrBadRequest, "display_name is too long")
	}
	note := strings.TrimSpace(input.Note)
	if len([]rune(note)) > maxProfileNoteLength {
		return "", "", nil, domainerrors.New(domainerrors.ErrBadRequest, "note is too long")
	}
	fields, derr := u.validatedProfileFields(input.Fields)
	if derr != nil {
		return "", "", nil, derr
	}
	return displayName, u.deps.ContentSanitizer.SanitizeHTML(note), fields, nil
}

func (u Profile) validatedProfileFields(input []models.AccountProfileField) ([]models.AccountProfileField, *domainerrors.DomainError) {
	if len(input) > maxProfileFields {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "too many profile fields")
	}
	fields := make([]models.AccountProfileField, 0, len(input))
	for _, field := range input {
		name := strings.TrimSpace(field.Name)
		value := strings.TrimSpace(field.Value)
		if name == "" && value == "" {
			continue
		}
		if len([]rune(name)) > maxProfileFieldNameLength {
			return nil, domainerrors.New(domainerrors.ErrBadRequest, "profile field name is too long")
		}
		if len([]rune(value)) > maxProfileFieldValueLength {
			return nil, domainerrors.New(domainerrors.ErrBadRequest, "profile field value is too long")
		}
		fields = append(fields, models.AccountProfileField{Name: name, Value: u.deps.ContentSanitizer.SanitizeHTML(value), VerifiedAt: field.VerifiedAt})
	}
	return fields, nil
}

func (u Profile) profileMediaIDs(ctx context.Context, account *models.Account, input UpdateCredentialsInput) (*string, *string, []models.MediaAttachment, *domainerrors.DomainError) {
	avatarMediaID := account.AvatarMediaID
	headerMediaID := account.HeaderMediaID
	createdMedia := []models.MediaAttachment{}
	if input.Avatar != nil {
		media, derr := u.createProfileMedia(ctx, account, *input.Avatar)
		if derr != nil {
			return nil, nil, nil, derr
		}
		createdMedia = append(createdMedia, *media)
		avatarMediaID = &media.ID
	}
	if input.Header != nil {
		media, derr := u.createProfileMedia(ctx, account, *input.Header)
		if derr != nil {
			u.deleteCreatedProfileMedia(ctx, createdMedia)
			return nil, nil, nil, derr
		}
		createdMedia = append(createdMedia, *media)
		headerMediaID = &media.ID
	}
	return avatarMediaID, headerMediaID, createdMedia, nil
}

func (u Profile) createProfileMedia(ctx context.Context, account *models.Account, input UploadMediaInput) (*models.MediaAttachment, *domainerrors.DomainError) {
	if len(input.Data) == 0 {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "profile image is required")
	}
	if len(input.Data) > MaxMediaUploadBytes {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "profile image is too large")
	}
	contentType, derr := safeProfileImageContentType(input.ContentType, input.Data)
	if derr != nil {
		return nil, derr
	}
	id, err := u.deps.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	storagePath, err := u.deps.MediaStorage.SaveMedia(ctx, id, input.FileName, input.Data)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	media, err := u.deps.MediaRepo.CreateMediaAttachment(ctx, nil, repos.CreateMediaAttachmentInput{LocalAccountID: account.ID, FileName: input.FileName, ContentType: contentType, StoragePath: storagePath, FileSize: int64(len(input.Data)), Description: input.Description})
	if err != nil {
		_ = u.deps.MediaStorage.DeleteMedia(ctx, storagePath)
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return media, nil
}

func safeProfileImageContentType(declared string, data []byte) (string, *domainerrors.DomainError) {
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	sniffed := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	allowed := map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true}
	if allowed[sniffed] {
		return sniffed, nil
	}
	if allowed[declared] {
		return declared, nil
	}
	return "", domainerrors.New(domainerrors.ErrBadRequest, "unsupported profile image content type")
}

func (u Profile) deleteCreatedProfileMedia(ctx context.Context, media []models.MediaAttachment) {
	for _, item := range media {
		_ = u.deleteMediaFilesAndRow(ctx, item)
	}
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
