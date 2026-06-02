package mastodon

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

const (
	maxProfileDisplayNameLength = 120
	maxProfileNoteLength        = 5000
)

func (u UseCase) UpdateCredentials(ctx context.Context, account *models.Account, input UpdateCredentialsInput) (*UpdateCredentialsResult, *domainerrors.DomainError) {
	if derr := requireAccount(account); derr != nil {
		return nil, derr
	}
	if account.UserID == nil {
		return nil, domainerrors.New(domainerrors.ErrUnauthorized, "only local accounts can update credentials")
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if len([]rune(displayName)) > maxProfileDisplayNameLength {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "display_name is too long")
	}
	note := strings.TrimSpace(input.Note)
	if len([]rune(note)) > maxProfileNoteLength {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "note is too long")
	}
	sanitizedNote := u.cfg.ContentSanitizer.SanitizeHTML(note)
	avatarMediaID := account.AvatarMediaID
	headerMediaID := account.HeaderMediaID
	oldAvatarMediaID := account.AvatarMediaID
	oldHeaderMediaID := account.HeaderMediaID
	createdMedia := []models.MediaAttachment{}
	if input.Avatar != nil {
		media, derr := u.createProfileMedia(ctx, account, *input.Avatar)
		if derr != nil {
			return nil, derr
		}
		createdMedia = append(createdMedia, *media)
		avatarMediaID = &media.ID
	}
	if input.Header != nil {
		media, derr := u.createProfileMedia(ctx, account, *input.Header)
		if derr != nil {
			u.deleteCreatedProfileMedia(ctx, createdMedia)
			return nil, derr
		}
		createdMedia = append(createdMedia, *media)
		headerMediaID = &media.ID
	}

	var updated *models.Account
	var raw []byte
	err := u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		var err error
		updated, err = u.cfg.AccountsRepo.UpdateLocalAccountProfile(ctx, &tx, account.ID, repos.UpdateAccountProfileInput{DisplayName: stringPtrOrNil(displayName), Summary: stringPtrOrNil(sanitizedNote), AvatarMediaID: avatarMediaID, HeaderMediaID: headerMediaID, AvatarURL: nil, HeaderURL: nil})
		if err != nil {
			return err
		}
		profileRaw, derr := u.profileUpdateActivity(*updated)
		if derr != nil {
			return derr
		}
		raw = profileRaw
		_, err = u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: updated.ID, Direction: models.ActivityDirectionOutbox, Type: "Update", Actor: updated.URI, Object: updated.URI, RawJSON: string(raw)})
		return err
	})
	if err != nil {
		u.deleteCreatedProfileMedia(ctx, createdMedia)
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	if input.Avatar != nil && oldAvatarMediaID != nil && *oldAvatarMediaID != *avatarMediaID {
		u.cleanupProfileMediaID(ctx, *oldAvatarMediaID)
	}
	if input.Header != nil && oldHeaderMediaID != nil && *oldHeaderMediaID != *headerMediaID {
		u.cleanupProfileMediaID(ctx, *oldHeaderMediaID)
	}
	inboxes, derr := u.followerInboxes(ctx, updated.ID)
	if derr != nil {
		return nil, derr
	}
	return &UpdateCredentialsResult{Account: *updated, RawJSON: raw, FollowerInboxes: inboxes}, nil
}

func (u UseCase) profileUpdateActivity(account models.Account) ([]byte, *domainerrors.DomainError) {
	activityID, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	actorJSON, err := u.cfg.ActorSerializer.Marshall(account)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	var actor map[string]any
	if err := json.Unmarshal([]byte(actorJSON), &actor); err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	delete(actor, "@context")
	now := time.Now().UTC().Format(time.RFC3339)
	activity := map[string]any{
		"@context":  "https://www.w3.org/ns/activitystreams",
		"id":        strings.TrimRight(account.URI, "/") + "/updates/" + activityID,
		"type":      "Update",
		"actor":     account.URI,
		"published": now,
		"to":        []string{"https://www.w3.org/ns/activitystreams#Public"},
		"cc":        []string{account.FollowersURI},
		"object":    actor,
	}
	raw, err := json.Marshal(activity)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return raw, nil
}

func (u UseCase) followerInboxes(ctx context.Context, localAccountID string) ([]string, *domainerrors.DomainError) {
	followers, err := u.cfg.FollowsRepo.ListFollowers(ctx, nil, localAccountID)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	inboxes := make([]string, 0, len(followers))
	for _, follower := range followers {
		if follower.RemoteInbox != nil {
			inboxes = append(inboxes, *follower.RemoteInbox)
		}
	}
	return inboxes, nil
}

func (u UseCase) createProfileMedia(ctx context.Context, account *models.Account, input UploadMediaInput) (*models.MediaAttachment, *domainerrors.DomainError) {
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
	id, err := u.cfg.IDGenerator.NewID()
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	storagePath, err := u.cfg.MediaStorage.SaveMedia(ctx, id, input.FileName, input.Data)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	media, err := u.cfg.MediaRepo.CreateMediaAttachment(ctx, nil, repos.CreateMediaAttachmentInput{LocalAccountID: account.ID, FileName: input.FileName, ContentType: contentType, StoragePath: storagePath, Description: input.Description})
	if err != nil {
		_ = u.cfg.MediaStorage.DeleteMedia(ctx, storagePath)
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

func (u UseCase) deleteCreatedProfileMedia(ctx context.Context, media []models.MediaAttachment) {
	for _, item := range media {
		_ = u.deleteMediaFilesAndRow(ctx, item)
	}
}

func (u UseCase) cleanupProfileMediaID(ctx context.Context, id string) {
	media, err := u.cfg.MediaRepo.GetMediaAttachmentByID(ctx, nil, id)
	if err != nil {
		return
	}
	attached, err := u.cfg.MediaRepo.MediaAttachmentIsAttached(ctx, nil, id)
	if err == nil && !attached {
		_ = u.deleteMediaFilesAndRow(ctx, *media)
	}
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
