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
					replyID, replyURI := replyIDs(ctx, u.cfg.NotesRepo, &tx, note)
					if err := enqueueMissingReplyFetch(ctx, u.cfg.FetchJobsRepo, &tx, account.ID, note, replyID); err != nil {
						return err
					}
					created, err := u.cfg.NotesRepo.CreateNote(ctx, &tx, repos.CreateNoteInput{LocalAccountID: account.ID, ActivityID: stored.ID, URI: note.URI, Content: u.cfg.ContentSanitizer.SanitizeHTML(note.Content), PlainText: u.cfg.ContentSanitizer.StripHTMLFromText(note.Content), Visibility: note.Visibility, Sensitive: note.Sensitive, SpoilerText: note.SpoilerText, AttributedTo: note.AttributedTo, InReplyToID: replyID, InReplyToURI: replyURI, PublishedAt: note.PublishedAt})
					if err != nil {
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
			if u.cfg.NotesRepo != nil {
				if note, ok := ExtractNoteObject(input.RawJSON); ok {
					if note.AttributedTo != "" && note.AttributedTo != activity.Actor {
						return domainerrors.New(domainerrors.ErrUnauthorized, "update actor does not own note")
					}
					if err := u.ensureRemoteNoteOwner(ctx, &tx, note.URI, activity.Actor); err != nil {
						return err
					}
					return u.cfg.NotesRepo.UpdateNoteByURI(ctx, &tx, note.URI, u.cfg.ContentSanitizer.SanitizeHTML(note.Content), u.cfg.ContentSanitizer.StripHTMLFromText(note.Content))
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
					if existing, err := u.cfg.RemoteAccountsRepo.GetRemoteAccountByURI(ctx, &tx, actorUpdate.URI); err == nil {
						mergeMissingRemoteAccountFields(&updatedAccount, *existing)
					}
					_, err := u.cfg.RemoteAccountsRepo.UpsertRemoteAccount(ctx, &tx, updatedAccount)
					return err
				}
			}
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
			if u.cfg.FollowsRepo != nil {
				remoteActor, err := ExtractUndoFollowActor(input.RawJSON)
				if err != nil {
					return domainerrors.NewErr(domainerrors.ErrBadRequest, err)
				}
				if remoteActor != "" {
					return u.cfg.FollowsRepo.DeleteFollowByActor(ctx, &tx, account.ID, remoteActor)
				}
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

func enqueueMissingAnnounceFetch(ctx context.Context, repo repos.FetchJobsRepository, tx *db.Tx, accountID, object string) error {
	_, err := repo.CreateFetchJob(ctx, tx, repos.CreateFetchJobInput{URL: object, Kind: "activitypub_object", AccountID: accountID, NextAttemptAt: time.Now().UTC()})
	return err
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
}
