package activitypub

import (
	"context"
	"database/sql"
	"errors"

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
			if err := u.cfg.FollowsRepo.AcceptFollow(ctx, &tx, follow.ID); err != nil {
				return err
			}
			acceptJSON, err = MarshalAccept(*account, *follow, input.RawJSON)
			return err
		case "Create":
			if u.cfg.NotesRepo != nil {
				if note, ok := ExtractNote(input.RawJSON); ok {
					replyID, replyURI := replyIDs(ctx, u.cfg.NotesRepo, &tx, note)
					if err := enqueueMissingReplyFetch(ctx, u.cfg.FetchJobsRepo, &tx, account.ID, note, replyID); err != nil {
						return err
					}
					_, err := u.cfg.NotesRepo.CreateNote(ctx, &tx, repos.CreateNoteInput{LocalAccountID: account.ID, ActivityID: stored.ID, URI: note.URI, Content: u.cfg.ContentSanitizer.SanitizeHTML(note.Content), PlainText: u.cfg.ContentSanitizer.StripHTMLFromText(note.Content), Visibility: note.Visibility, Sensitive: note.Sensitive, SpoilerText: note.SpoilerText, AttributedTo: note.AttributedTo, InReplyToID: replyID, InReplyToURI: replyURI, PublishedAt: note.PublishedAt})
					return err
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

func (u *HandleInboxActivityUseCase) ensureRemoteNoteOwner(ctx context.Context, tx *db.Tx, uri string, actor string) error {
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

func validateFollowResponseObject(raw []byte, localActor string, remoteActor string) error {
	follow, ok, err := ExtractFollowObject(raw)
	if err != nil {
		return domainerrors.NewErr(domainerrors.ErrBadRequest, err)
	}
	if !ok || follow.Actor != localActor || follow.Object != remoteActor {
		return domainerrors.New(domainerrors.ErrBadRequest, "accept/reject object does not match local follow")
	}
	return nil
}
