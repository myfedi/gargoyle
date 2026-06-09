package activitypub

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

// HandleInboxActivityInput contains a remote activity addressed to a local actor.
type HandleInboxActivityInput struct {
	Username string
	RawJSON  []byte
	Inbox    string
}

// HandleInboxActivityResult contains committed inbound processing state and an
// optional Accept activity that infrastructure may deliver after commit.
type HandleInboxActivityResult struct {
	Account     models.Account
	Activity    ParsedActivity
	AcceptJSON  []byte
	AcceptInbox string
}

// ResolveSharedInboxRecipients determines which local actors should process a
// shared-inbox activity. Explicit local recipients are included, and follower
// delivery is expanded from Gargoyle's local follow state because remote servers
// may deliver one followers-addressed activity to the shared inbox without
// naming each local follower actor individually.
func (u *HandleInboxActivityUseCase) ResolveSharedInboxRecipients(ctx context.Context, raw []byte, actor string) ([]string, *domainerrors.DomainError) {
	usernames := ExtractLocalRecipientUsernames(raw, u.cfg.Host)
	seen := make(map[string]bool, len(usernames))
	for _, username := range usernames {
		seen[username] = true
	}
	follows, err := u.cfg.FollowsRepo.ListLocalFollowersOfRemoteActor(ctx, nil, actor)
	if err != nil {
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	for _, follow := range follows {
		account, err := u.cfg.AccountsRepo.GetAccountByID(ctx, nil, follow.LocalAccountID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
		}
		if account.Username == "" || seen[account.Username] {
			continue
		}
		seen[account.Username] = true
		usernames = append(usernames, account.Username)
	}
	return usernames, nil
}

// InspectInboxActivity loads the addressed local actor and parses the raw
// activity before infrastructure performs authentication checks.
func (u *HandleInboxActivityUseCase) InspectInboxActivity(ctx context.Context, username string, raw []byte) (*models.Account, ParsedActivity, *domainerrors.DomainError) {
	account, derr := localAccount(ctx, u.cfg.AccountsRepo, username)
	if derr != nil {
		return nil, ParsedActivity{}, derr
	}
	activity, derr := ParseActivity(raw)
	if derr != nil {
		return nil, ParsedActivity{}, derr
	}
	return account, activity, nil
}

// HandleInboxActivity stores the inbound activity and applies all derived local
// state changes in one transaction. Network side effects are returned, not performed.
func (u *HandleInboxActivityUseCase) HandleInboxActivity(ctx context.Context, input HandleInboxActivityInput) (*HandleInboxActivityResult, *domainerrors.DomainError) {
	account, activity, derr := u.InspectInboxActivity(ctx, input.Username, input.RawJSON)
	if derr != nil {
		return nil, derr
	}

	// Validate before writing when possible.
	if activity.Actor == "" {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "activity actor is required")
	}
	if u.cfg.DomainBlocksRepo != nil {
		if domain := actorDomain(activity.Actor); domain != "" {
			blocked, err := u.cfg.DomainBlocksRepo.DomainIsSuspended(ctx, nil, domain)
			if err != nil {
				return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
			}
			if blocked {
				return nil, domainerrors.New(domainerrors.ErrUnauthorized, "remote domain is suspended")
			}
		}
	}
	if activity.Type == "Follow" && activity.Object != account.URI {
		return nil, domainerrors.New(domainerrors.ErrBadRequest, "follow object does not match local actor")
	}

	var acceptJSON []byte
	acceptInbox := input.Inbox
	if activity.Type == "Follow" && acceptInbox == "" && u.cfg.ActorFetcher != nil {
		actor, err := u.cfg.ActorFetcher.FetchActor(ctx, activity.Actor, account)
		if err == nil && actor != nil {
			acceptInbox = actor.Inbox
		}
	}
	err := u.cfg.TxProvider.RunInTx(ctx, sql.TxOptions{}, func(ctx context.Context, tx db.Tx) error {
		stored, err := u.cfg.ActivitiesRepo.CreateActivity(ctx, &tx, repos.CreateActivityInput{LocalAccountID: account.ID, Direction: models.ActivityDirectionInbox, Type: activity.Type, Actor: activity.Actor, Object: activity.Object, RawJSON: string(input.RawJSON)})
		if err != nil {
			return err
		}

		switch activity.Type {
		case "Follow":
			if u.cfg.FollowsRepo == nil {
				return nil
			}
			var inboxPtr *string
			if acceptInbox != "" {
				inboxPtr = &acceptInbox
			}
			follow, err := u.cfg.FollowsRepo.CreateFollow(ctx, &tx, repos.CreateFollowInput{LocalAccountID: account.ID, RemoteActor: activity.Actor, RemoteInbox: inboxPtr, ActivityID: stored.ID})
			if err != nil {
				return err
			}
			if account.Locked {
				if u.cfg.SocialRepo != nil {
					_, err := u.cfg.SocialRepo.CreateNotification(ctx, &tx, account.ID, activity.Actor, "follow_request", nil)
					return err
				}
				return nil
			}
			if err := u.cfg.FollowsRepo.AcceptFollow(ctx, &tx, follow.ID); err != nil {
				return err
			}
			if u.cfg.SocialRepo != nil {
				if _, err := u.cfg.SocialRepo.CreateNotification(ctx, &tx, account.ID, activity.Actor, "follow", nil); err != nil {
					return err
				}
			}
			acceptJSON, err = MarshalAccept(*account, *follow, input.RawJSON)
			return err
		case "Create":
			if u.cfg.NotesRepo != nil {
				if note, ok := ExtractNote(input.RawJSON); ok {
					if note.AttributedTo == "" || note.AttributedTo != activity.Actor {
						return domainerrors.New(domainerrors.ErrUnauthorized, "create actor does not own note")
					}
					if handled, err := u.handlePollVoteCreate(ctx, &tx, note); handled || err != nil {
						return err
					}
					replyID, replyURI := replyIDs(ctx, u.cfg.NotesRepo, &tx, note)
					if err := enqueueMissingReplyFetch(ctx, u.cfg.FetchJobsRepo, &tx, account.ID, note, replyID); err != nil {
						return err
					}
					created, err := u.cfg.NotesRepo.CreateNote(ctx, &tx, repos.CreateNoteInput{LocalAccountID: account.ID, ActivityID: stored.ID, URI: note.URI, Content: u.cfg.ContentSanitizer.SanitizeHTML(note.Content), PlainText: u.cfg.ContentSanitizer.StripHTMLFromText(note.Content), ObjectType: note.Type, PollMultiple: note.PollMultiple, PollExpiresAt: note.PollExpiresAt, Hashtags: note.Hashtags, Emojis: note.Emojis, Visibility: note.Visibility, Sensitive: note.Sensitive, SpoilerText: note.SpoilerText, AttributedTo: note.AttributedTo, InReplyToID: replyID, InReplyToURI: replyURI, PublishedAt: note.PublishedAt})
					if err != nil {
						return err
					}
					if err := u.createPollOptions(ctx, &tx, created.ID, note); err != nil {
						return err
					}
					return u.createStatusNotification(ctx, &tx, *account, note, created.ID, replyID)
				}
			}
		case "Delete":
			if u.cfg.NotesRepo != nil && activity.Object != "" {
				if err := u.ensureRemoteNoteOwner(ctx, &tx, activity.Object, activity.Actor); err != nil {
					return err
				}
				return u.cfg.NotesRepo.DeleteNoteByURI(ctx, &tx, activity.Object)
			}
		case "Update":
			if tombstoneID := ExtractObjectIDByType(input.RawJSON, "Tombstone"); tombstoneID != "" && u.cfg.NotesRepo != nil {
				if err := u.ensureRemoteNoteOwner(ctx, &tx, tombstoneID, activity.Actor); err != nil {
					return err
				}
				return u.cfg.NotesRepo.DeleteNoteByURI(ctx, &tx, tombstoneID)
			}
			if u.cfg.NotesRepo != nil {
				if note, ok := ExtractNoteObject(input.RawJSON); ok {
					if note.AttributedTo != "" && note.AttributedTo != activity.Actor {
						return domainerrors.New(domainerrors.ErrUnauthorized, "update actor does not own note")
					}
					if err := u.ensureRemoteNoteOwner(ctx, &tx, note.URI, activity.Actor); err != nil {
						return err
					}
					return u.cfg.NotesRepo.UpdateNoteByURI(ctx, &tx, note.URI, u.cfg.ContentSanitizer.SanitizeHTML(note.Content), u.cfg.ContentSanitizer.StripHTMLFromText(note.Content), note.Type)
				}
			}
			if u.cfg.RemoteAccountsRepo != nil {
				if actorUpdate, ok := ExtractActorObject(input.RawJSON); ok {
					if actorUpdate.Inbox == "" {
						return domainerrors.New(domainerrors.ErrBadRequest, "actor update is missing inbox")
					}
					if actorUpdate.URI != activity.Actor {
						return domainerrors.New(domainerrors.ErrUnauthorized, "update actor does not own actor object")
					}
					updatedAccount := accountFromExtractedActor(actorUpdate)
					updatedAccount.Fields = sanitizeAccountProfileFields(u.cfg.ContentSanitizer, updatedAccount.Fields)
					if existing, err := u.cfg.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, &tx, actorUpdate.URI); err == nil {
						mergeMissingRemoteAccountFields(&updatedAccount, *existing)
					}
					_, err := u.cfg.RemoteAccountsRepo.UpsertRemoteAccount(ctx, &tx, updatedAccount)
					return err
				}
			}
		case "Move":
			return u.handleMoveActivity(ctx, &tx, *account, activity, input.RawJSON)
		case "Flag":
			// The raw Flag activity is already stored above. Moderation workflows can
			// surface stored Flag activities without applying automatic punishment here.
			return nil
		case "Block":
			return u.handleBlockActivity(ctx, &tx, *account, activity)
		case "Like":
			return u.createInteractionNotification(ctx, &tx, *account, activity, "favourite")
		case "Announce":
			if err := u.createInboundBoost(ctx, &tx, *account, activity); err != nil {
				return err
			}
			return u.createInteractionNotification(ctx, &tx, *account, activity, "reblog")
		case "Accept":
			if u.cfg.FollowsRepo != nil && activity.Actor != "" {
				if err := validateFollowResponseObject(input.RawJSON, account.URI, activity.Actor); err != nil {
					return err
				}
				return u.cfg.FollowsRepo.AcceptFollowingByActor(ctx, &tx, account.ID, activity.Actor)
			}
		case "Reject":
			if u.cfg.FollowsRepo != nil && activity.Actor != "" {
				if err := validateFollowResponseObject(input.RawJSON, account.URI, activity.Actor); err != nil {
					return err
				}
				return u.cfg.FollowsRepo.RejectFollowingByActor(ctx, &tx, account.ID, activity.Actor)
			}
		case "Undo":
			undo, err := u.resolveUndoActivity(ctx, &tx, account.ID, activity.Actor, input.RawJSON)
			if err != nil {
				return err
			}
			switch undo.Type {
			case "Follow":
				if u.cfg.FollowsRepo != nil && undo.Actor != "" {
					return u.cfg.FollowsRepo.DeleteFollowByActor(ctx, &tx, account.ID, undo.Actor)
				}
			case "Like":
				return u.undoInteractionNotification(ctx, &tx, *account, undo, "favourite")
			case "Announce":
				if err := u.undoInboundBoost(ctx, &tx, *account, undo); err != nil {
					return err
				}
				return u.undoInteractionNotification(ctx, &tx, *account, undo, "reblog")
			}
		}
		return nil
	})
	if err != nil {
		var derr *domainerrors.DomainError
		if errors.As(err, &derr) {
			return nil, derr
		}
		return nil, domainerrors.NewErr(domainerrors.ErrInternal, err)
	}
	return &HandleInboxActivityResult{Account: *account, Activity: activity, AcceptJSON: acceptJSON, AcceptInbox: acceptInbox}, nil
}

func (u *HandleInboxActivityUseCase) handleMoveActivity(ctx context.Context, tx *db.Tx, account models.Account, activity ParsedActivity, raw []byte) error {
	if activity.Object != activity.Actor {
		return domainerrors.New(domainerrors.ErrUnauthorized, "move actor does not own object")
	}
	target := ExtractMoveTarget(raw)
	if target == "" || u.cfg.ActorFetcher == nil {
		return nil
	}
	fetched, err := u.cfg.ActorFetcher.FetchActor(ctx, target, &account)
	if err != nil || fetched == nil || fetched.Inbox == "" {
		return nil
	}
	// A valid Move is acknowledged only as an observed event for now. Rewriting
	// follow relationships is intentionally deferred until aliases/alsoKnownAs are
	// modelled and validated, otherwise Move can become an account-takeover vector.
	return nil
}

func (u *HandleInboxActivityUseCase) handleBlockActivity(ctx context.Context, tx *db.Tx, account models.Account, activity ParsedActivity) error {
	if u.cfg.FollowsRepo == nil || activity.Actor == "" {
		return nil
	}
	// If a remote actor blocks the local actor, remove both relationship rows so
	// we stop delivering to, and accepting timeline assumptions about, that actor.
	if activity.Object != "" && activity.Object != account.URI {
		return nil
	}
	if err := u.cfg.FollowsRepo.DeleteFollowingByActor(ctx, tx, account.ID, activity.Actor); err != nil {
		return err
	}
	return u.cfg.FollowsRepo.DeleteFollowByActor(ctx, tx, account.ID, activity.Actor)
}

func (u *HandleInboxActivityUseCase) handlePollVoteCreate(ctx context.Context, tx *db.Tx, note ExtractedNote) (bool, error) {
	if u.cfg.PollsRepo == nil || u.cfg.NotesRepo == nil || note.InReplyToURI == nil || note.Content == "" {
		return false, nil
	}
	parent, err := u.cfg.NotesRepo.GetNoteByURI(ctx, tx, *note.InReplyToURI)
	if err != nil || parent.ObjectType != "Question" {
		return false, nil
	}
	_, err = u.cfg.PollsRepo.CreateRemoteVote(ctx, tx, parent.ID, note.AttributedTo, note.Content, parent.PollMultiple)
	if err != nil {
		return true, err
	}
	return true, nil
}

func (u *HandleInboxActivityUseCase) createStatusNotification(ctx context.Context, tx *db.Tx, account models.Account, note ExtractedNote, statusID string, replyID *string) error {
	if u.cfg.SocialRepo == nil {
		return nil
	}
	statusIDPtr := &statusID
	if noteMentionsLocalActor(note, account.URI) {
		_, err := u.cfg.SocialRepo.CreateNotification(ctx, tx, account.ID, note.AttributedTo, "mention", statusIDPtr)
		return err
	}
	if replyID == nil {
		return nil
	}
	parent, err := u.cfg.NotesRepo.GetNoteByID(ctx, tx, *replyID)
	if err != nil {
		return nil
	}
	if parent.AttributedTo != account.URI {
		return nil
	}
	_, err = u.cfg.SocialRepo.CreateNotification(ctx, tx, account.ID, note.AttributedTo, "status", statusIDPtr)
	return err
}

func noteMentionsLocalActor(note ExtractedNote, localActor string) bool {
	for _, uri := range note.MentionURIs {
		if uri == localActor {
			return true
		}
	}
	for _, uri := range note.To {
		if uri == localActor {
			return true
		}
	}
	for _, uri := range note.CC {
		if uri == localActor {
			return true
		}
	}
	return false
}

func (u *HandleInboxActivityUseCase) resolveUndoActivity(ctx context.Context, tx *db.Tx, localAccountID, outerActor string, raw []byte) (ExtractedUndoActivity, error) {
	undo, err := ExtractUndoActivity(raw)
	if err != nil {
		return ExtractedUndoActivity{}, domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	if undo.Type == "" && undo.Object != "" && u.cfg.ActivitiesRepo != nil {
		stored, err := u.cfg.ActivitiesRepo.GetActivityByURI(ctx, tx, localAccountID, undo.Object)
		if err != nil {
			return ExtractedUndoActivity{}, nil
		}
		parsed, derr := ParseActivity([]byte(stored.RawJSON))
		if derr != nil {
			return ExtractedUndoActivity{}, derr
		}
		undo = ExtractedUndoActivity{Type: parsed.Type, Actor: parsed.Actor, Object: parsed.Object}
	}
	if undo.Type == "" {
		return undo, nil
	}
	if undo.Actor != outerActor {
		return ExtractedUndoActivity{}, domainerrors.New(domainerrors.ErrUnauthorized, "undo actor does not own embedded activity")
	}
	return undo, nil
}

func (u *HandleInboxActivityUseCase) createInboundBoost(ctx context.Context, tx *db.Tx, account models.Account, activity ParsedActivity) error {
	if u.cfg.BoostsRepo == nil || u.cfg.NotesRepo == nil || activity.Object == "" {
		return nil
	}
	note, err := u.cfg.NotesRepo.GetNoteByURI(ctx, tx, activity.Object)
	if err != nil {
		if u.cfg.FetchJobsRepo != nil {
			return enqueueMissingAnnounceFetch(ctx, u.cfg.FetchJobsRepo, tx, account.ID, activity.Object)
		}
		return nil
	}
	_, err = u.cfg.BoostsRepo.CreateBoost(ctx, tx, repos.CreateBoostInput{LocalAccountID: account.ID, Actor: activity.Actor, NoteID: note.ID, URI: activity.Object + "#announce-" + activity.Actor, PublishedAt: note.PublishedAt})
	return err
}

func (u *HandleInboxActivityUseCase) undoInboundBoost(ctx context.Context, tx *db.Tx, account models.Account, undo ExtractedUndoActivity) error {
	if u.cfg.BoostsRepo == nil || u.cfg.NotesRepo == nil || undo.Object == "" {
		return nil
	}
	note, err := u.cfg.NotesRepo.GetNoteByURI(ctx, tx, undo.Object)
	if err != nil {
		return nil
	}
	return u.cfg.BoostsRepo.DeleteBoost(ctx, tx, account.ID, undo.Actor, note.ID)
}

func enqueueMissingAnnounceFetch(ctx context.Context, repo repos.FetchJobsRepository, tx *db.Tx, accountID, object string) error {
	_, err := repo.CreateFetchJob(ctx, tx, repos.CreateFetchJobInput{URL: object, Kind: "activitypub_object", AccountID: accountID, NextAttemptAt: time.Now().UTC()})
	return err
}

func (u *HandleInboxActivityUseCase) undoInteractionNotification(ctx context.Context, tx *db.Tx, account models.Account, undo ExtractedUndoActivity, notificationType string) error {
	if u.cfg.SocialRepo == nil || undo.Object == "" {
		return nil
	}
	return u.cfg.SocialRepo.DeleteNotificationsByActorAndType(ctx, tx, account.ID, undo.Actor, notificationType)
}

func (u *HandleInboxActivityUseCase) createInteractionNotification(ctx context.Context, tx *db.Tx, account models.Account, activity ParsedActivity, notificationType string) error {
	if u.cfg.SocialRepo == nil || u.cfg.NotesRepo == nil || activity.Object == "" {
		return nil
	}
	note, err := u.cfg.NotesRepo.GetNoteByURI(ctx, tx, activity.Object)
	if err != nil || note.AttributedTo != account.URI {
		return nil
	}
	_, err = u.cfg.SocialRepo.CreateNotification(ctx, tx, account.ID, activity.Actor, notificationType, &note.ID)
	return err
}

func (u *HandleInboxActivityUseCase) ensureRemoteNoteOwner(ctx context.Context, tx *db.Tx, uri, actor string) error {
	note, err := u.cfg.NotesRepo.GetNoteByURI(ctx, tx, uri)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainerrors.New(domainerrors.ErrNotFound, "note does not exist")
		}
		return err
	}
	if note.AttributedTo != actor {
		return domainerrors.New(domainerrors.ErrUnauthorized, "activity actor does not own note")
	}
	return nil
}

func validateFollowResponseObject(raw []byte, localActor, remoteActor string) error {
	follow, ok, err := ExtractFollowObject(raw)
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	if !ok || follow.Actor != localActor || follow.Object != remoteActor {
		return domainerrors.New(domainerrors.ErrBadRequest, "accept/reject object does not match local follow")
	}
	return nil
}

func actorDomain(actor string) string {
	parsed, err := url.Parse(strings.TrimSpace(actor))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func accountFromExtractedActor(actor ExtractedActor) models.Account {
	domain := ""
	if parsed, err := url.Parse(actor.URI); err == nil {
		domain = parsed.Host
	}
	return models.Account{
		ID:           remoteAccountID(actor.URI),
		Username:     actor.Username,
		Domain:       stringPtr(domain),
		DisplayName:  stringPtr(firstNonEmpty(actor.Name, actor.Username)),
		Summary:      stringPtr(actor.Summary),
		URI:          actor.URI,
		URL:          stringPtr(firstNonEmpty(actor.URL, actor.URI)),
		Fields:       actor.Fields,
		AvatarURL:    stringPtr(actor.AvatarURL),
		HeaderURL:    stringPtr(actor.HeaderURL),
		InboxURI:     actor.Inbox,
		OutboxURI:    stringPtr(actor.Outbox),
		FollowingURI: actor.Following,
		FollowersURI: actor.Followers,
		PublicKey:    actor.PublicKey,
		ActorType:    actorTypeFromString(actor.Type),
		Locked:       actor.Locked,
	}
}

func remoteAccountID(actor string) string {
	return remoteAccountIDPrefix + base64.RawURLEncoding.EncodeToString([]byte(actor))
}

func actorTypeFromString(value string) models.ActorType {
	switch value {
	case "Application":
		return models.ActorTypeApplication
	case "Group":
		return models.ActorTypeGroup
	case "Organization":
		return models.ActorTypeOrganization
	case "Service":
		return models.ActorTypeService
	default:
		return models.ActorTypePerson
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func sanitizeAccountProfileFields(sanitizer ports.ContentSanitizer, fields []models.AccountProfileField) []models.AccountProfileField {
	if sanitizer == nil || len(fields) == 0 {
		return fields
	}
	res := make([]models.AccountProfileField, 0, len(fields))
	for _, field := range fields {
		res = append(res, models.AccountProfileField{Name: field.Name, Value: sanitizer.SanitizeHTML(field.Value), VerifiedAt: field.VerifiedAt})
	}
	return res
}

func mergeMissingRemoteAccountFields(account *models.Account, existing models.Account) {
	if account.PublicKey == "" {
		account.PublicKey = existing.PublicKey
	}
	if account.OutboxURI == nil {
		account.OutboxURI = existing.OutboxURI
	}
	if account.FollowingURI == "" {
		account.FollowingURI = existing.FollowingURI
	}
	if account.FollowersURI == "" {
		account.FollowersURI = existing.FollowersURI
	}
	if account.URL == nil {
		account.URL = existing.URL
	}
	if account.AvatarURL == nil {
		account.AvatarURL = existing.AvatarURL
	}
	if account.HeaderURL == nil {
		account.HeaderURL = existing.HeaderURL
	}
	if len(account.Fields) == 0 && len(existing.Fields) > 0 {
		account.Fields = existing.Fields
	}
}
