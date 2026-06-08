package activitypub

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/myfedi/gargoyle/adapters"
	apAdapters "github.com/myfedi/gargoyle/adapters/activitypub"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/models/domainerrors"
	activitypubPorts "github.com/myfedi/gargoyle/domain/ports/activitypub"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type fakeDeliveryJobsRepo struct{}

func (f fakeDeliveryJobsRepo) CreateDeliveryJob(ctx context.Context, tx *db.Tx, input repos.CreateDeliveryJobInput) (*models.DeliveryJob, error) {
	return &models.DeliveryJob{ID: "job-1", AccountID: input.AccountID, InboxURL: input.InboxURL, Payload: input.Payload, NextAttemptAt: input.NextAttemptAt}, nil
}
func (f fakeDeliveryJobsRepo) ClaimDueDeliveryJobs(ctx context.Context, tx *db.Tx, now time.Time, limit int) ([]models.DeliveryJob, error) {
	return nil, nil
}
func (f fakeDeliveryJobsRepo) ListDueDeliveryJobs(ctx context.Context, tx *db.Tx, now time.Time, limit int) ([]models.DeliveryJob, error) {
	return nil, nil
}
func (f fakeDeliveryJobsRepo) ListDeliveryJobsByStatus(ctx context.Context, tx *db.Tx, status models.JobStatus, limit int) ([]models.DeliveryJob, error) {
	return nil, nil
}
func (f fakeDeliveryJobsRepo) MarkDeliveryJobDelivered(ctx context.Context, tx *db.Tx, id string, deliveredAt time.Time) error {
	return nil
}
func (f fakeDeliveryJobsRepo) MarkDeliveryJobFailed(ctx context.Context, tx *db.Tx, id string, attempts int, nextAttemptAt time.Time, lastError string) error {
	return nil
}

type acceptingSignatureVerifier struct {
	actors []string
}

func (f *acceptingSignatureVerifier) VerifyInbound(ctx context.Context, input activitypubPorts.SignatureVerificationInput) *domainerrors.DomainError {
	f.actors = append(f.actors, input.Actor)
	return nil
}

type fakeActorFetcher struct {
	fetched []string
}

func (f *fakeActorFetcher) FetchActor(ctx context.Context, actor string, signer *models.Account) (*activitypubPorts.RemoteActorDocument, error) {
	f.fetched = append(f.fetched, actor)
	return &activitypubPorts.RemoteActorDocument{Inbox: actor + "/inbox"}, nil
}

type fakeFetchJobsRepo struct{}

func (fakeFetchJobsRepo) CreateFetchJob(ctx context.Context, tx *db.Tx, input repos.CreateFetchJobInput) (*models.FetchJob, error) {
	return &models.FetchJob{ID: "fetch-1", URL: input.URL, Kind: input.Kind, AccountID: input.AccountID, NextAttemptAt: input.NextAttemptAt}, nil
}
func (fakeFetchJobsRepo) ClaimDueFetchJobs(ctx context.Context, tx *db.Tx, now time.Time, limit int) ([]models.FetchJob, error) {
	return nil, nil
}
func (fakeFetchJobsRepo) ListDueFetchJobs(ctx context.Context, tx *db.Tx, now time.Time, limit int) ([]models.FetchJob, error) {
	return nil, nil
}
func (fakeFetchJobsRepo) ListFetchJobsByStatus(ctx context.Context, tx *db.Tx, status models.JobStatus, limit int) ([]models.FetchJob, error) {
	return nil, nil
}
func (fakeFetchJobsRepo) MarkFetchJobFetched(ctx context.Context, tx *db.Tx, id string, fetchedAt time.Time) error {
	return nil
}
func (fakeFetchJobsRepo) MarkFetchJobFailed(ctx context.Context, tx *db.Tx, id string, attempts int, nextAttemptAt time.Time, lastError string) error {
	return nil
}

type fakeAccountsRepo struct{ err error }

func (f fakeAccountsRepo) CreateAccount(ctx context.Context, tx *db.Tx, input repos.CreateAccountInput) (*models.Account, error) {
	return nil, nil
}
func (f fakeAccountsRepo) UpdateLocalAccountProfile(ctx context.Context, tx *db.Tx, id string, input repos.UpdateAccountProfileInput) (*models.Account, error) {
	return nil, nil
}
func (f fakeAccountsRepo) GetAccountByID(ctx context.Context, tx *db.Tx, id string) (*models.Account, error) {
	return nil, sql.ErrNoRows
}

func (f fakeAccountsRepo) GetAccountByUserID(ctx context.Context, tx *db.Tx, userID string) (*models.Account, error) {
	return nil, nil
}
func (f fakeAccountsRepo) GetLocalAccountByUsername(ctx context.Context, tx *db.Tx, username string) (*models.Account, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &models.Account{
		ID:           "account-1",
		Username:     username,
		URI:          "https://example.org/users/" + username,
		InboxURI:     "https://example.org/users/" + username + "/inbox",
		FollowersURI: "https://example.org/users/" + username + "/followers",
		FollowingURI: "https://example.org/users/" + username + "/following",
		PublicKey:    "public-key-pem",
		ActorType:    models.ActorTypePerson,
	}, nil
}
func (f fakeAccountsRepo) SearchLocalAccounts(ctx context.Context, tx *db.Tx, query string, limit int) ([]models.Account, error) {
	account, err := f.GetLocalAccountByUsername(ctx, tx, query)
	if err != nil {
		return nil, err
	}
	return []models.Account{*account}, nil
}
func (f fakeAccountsRepo) AccountWithUsernameExists(ctx context.Context, tx *db.Tx, username string) (bool, error) {
	return false, nil
}

type fakeActivitiesRepo struct{ activities []models.Activity }

func (f *fakeActivitiesRepo) CreateActivity(ctx context.Context, tx *db.Tx, input repos.CreateActivityInput) (*models.Activity, error) {
	activity := models.Activity{ID: "activity-1", LocalAccountID: input.LocalAccountID, Direction: input.Direction, Type: input.Type, Actor: input.Actor, Object: input.Object, RawJSON: input.RawJSON}
	f.activities = append(f.activities, activity)
	return &activity, nil
}
func (f *fakeActivitiesRepo) ListOutboxActivities(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Activity, error) {
	return f.activities, nil
}
func (f *fakeActivitiesRepo) GetActivityByID(ctx context.Context, tx *db.Tx, id string) (*models.Activity, error) {
	for _, activity := range f.activities {
		if activity.ID == id {
			return &activity, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (f *fakeActivitiesRepo) GetActivityByURI(ctx context.Context, tx *db.Tx, localAccountID, uri string) (*models.Activity, error) {
	for _, activity := range f.activities {
		var doc struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal([]byte(activity.RawJSON), &doc)
		if activity.LocalAccountID == localAccountID && doc.ID == uri {
			return &activity, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (f *fakeActivitiesRepo) GetOutboxActivityByURI(ctx context.Context, tx *db.Tx, localAccountID, uri string) (*models.Activity, error) {
	activity, err := f.GetActivityByURI(ctx, tx, localAccountID, uri)
	if err != nil {
		return nil, err
	}
	if activity.Direction != models.ActivityDirectionOutbox {
		return nil, sql.ErrNoRows
	}
	return activity, nil
}
func (f *fakeActivitiesRepo) ListOutboxActivitiesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit, offset int) ([]models.Activity, error) {
	if limit <= 0 {
		return f.activities, nil
	}
	if offset >= len(f.activities) {
		return []models.Activity{}, nil
	}
	end := offset + limit
	if end > len(f.activities) {
		end = len(f.activities)
	}
	return f.activities[offset:end], nil
}
func (f *fakeActivitiesRepo) ListPublicOutboxActivitiesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit, offset int) ([]models.Activity, error) {
	return f.ListOutboxActivitiesPaged(ctx, tx, localAccountID, limit, offset)
}
func (f *fakeActivitiesRepo) CountOutboxActivities(ctx context.Context, tx *db.Tx, localAccountID string) (int, error) {
	return len(f.activities), nil
}
func (f *fakeActivitiesRepo) CountPublicOutboxActivities(ctx context.Context, tx *db.Tx, localAccountID string) (int, error) {
	return len(f.activities), nil
}

type fakeNotesRepo struct{ notes []models.Note }

func (f *fakeNotesRepo) GetLocalPostsCount(ctx context.Context) (int, error) {
	return len(f.notes), nil
}
func (f *fakeNotesRepo) CreateNote(ctx context.Context, tx *db.Tx, input repos.CreateNoteInput) (*models.Note, error) {
	note := models.Note{ID: "note-1", LocalAccountID: input.LocalAccountID, ActivityID: input.ActivityID, URI: input.URI, Content: input.Content, PlainText: input.PlainText, ObjectType: input.ObjectType, Visibility: input.Visibility, Sensitive: input.Sensitive, SpoilerText: input.SpoilerText, AttributedTo: input.AttributedTo, InReplyToID: input.InReplyToID, InReplyToURI: input.InReplyToURI, PublishedAt: input.PublishedAt}
	f.notes = append(f.notes, note)
	return &note, nil
}
func (f *fakeNotesRepo) GetNoteByID(ctx context.Context, tx *db.Tx, id string) (*models.Note, error) {
	for _, note := range f.notes {
		if note.ID == id {
			return &note, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (f *fakeNotesRepo) GetNoteByURI(ctx context.Context, tx *db.Tx, uri string) (*models.Note, error) {
	for _, note := range f.notes {
		if note.URI == uri {
			return &note, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (f *fakeNotesRepo) UpdateNoteByID(ctx context.Context, tx *db.Tx, id string, input repos.UpdateNoteInput) (*models.Note, error) {
	for i := range f.notes {
		if f.notes[i].ID == id {
			f.notes[i].Content = input.Content
			f.notes[i].PlainText = input.PlainText
			f.notes[i].ObjectType = input.ObjectType
			f.notes[i].Visibility = input.Visibility
			f.notes[i].Sensitive = input.Sensitive
			f.notes[i].SpoilerText = input.SpoilerText
			return &f.notes[i], nil
		}
	}
	return nil, sql.ErrNoRows
}

func (f *fakeNotesRepo) UpdateNoteByURI(ctx context.Context, tx *db.Tx, uri, content, plainText, objectType string) error {
	for i := range f.notes {
		if f.notes[i].URI == uri {
			f.notes[i].Content = content
			f.notes[i].PlainText = plainText
			f.notes[i].ObjectType = objectType
		}
	}
	return nil
}
func (f *fakeNotesRepo) CreateNoteEdit(ctx context.Context, tx *db.Tx, input repos.CreateNoteEditInput) (*models.NoteEdit, error) {
	return &models.NoteEdit{ID: "edit-1", NoteID: input.Note.ID, Content: input.Note.Content, PlainText: input.Note.PlainText, ObjectType: input.Note.ObjectType, Visibility: input.Note.Visibility, Sensitive: input.Note.Sensitive, SpoilerText: input.Note.SpoilerText, CreatedAt: input.CreatedAt, MediaIDs: input.MediaIDs}, nil
}
func (f *fakeNotesRepo) ListNoteEdits(ctx context.Context, tx *db.Tx, noteID string) ([]models.NoteEdit, error) {
	return nil, nil
}
func (f *fakeNotesRepo) DeleteNoteByID(ctx context.Context, tx *db.Tx, id string) error {
	for i, note := range f.notes {
		if note.ID == id {
			f.notes = append(f.notes[:i], f.notes[i+1:]...)
			return nil
		}
	}
	return nil
}
func (f *fakeNotesRepo) DeleteNoteByURI(ctx context.Context, tx *db.Tx, uri string) error {
	for i, note := range f.notes {
		if note.URI == uri {
			f.notes = append(f.notes[:i], f.notes[i+1:]...)
			return nil
		}
	}
	return nil
}
func (f *fakeNotesRepo) ListLocalNotes(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListLocalNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListHomeTimelineNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, actorURIs []string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListDirectNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListKnownPublicTimelineNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListKnownLocalTimelineNotesPaged(ctx context.Context, tx *db.Tx, localAccountID, localActorPrefix string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListKnownRemoteTimelineNotesPaged(ctx context.Context, tx *db.Tx, localAccountID, localActorPrefix string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListAttributedNotesPaged(ctx context.Context, tx *db.Tx, localAccountID, attributedTo string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListReplies(ctx context.Context, tx *db.Tx, localAccountID, parentID, parentURI string) ([]models.Note, error) {
	return f.notes, nil
}

type fakeFollowsRepo struct{ followers []models.Follow }

func (f *fakeFollowsRepo) CreateFollow(ctx context.Context, tx *db.Tx, input repos.CreateFollowInput) (*models.Follow, error) {
	direction := input.Direction
	if direction == "" {
		direction = "follower"
	}
	follow := models.Follow{ID: "follow-1", LocalAccountID: input.LocalAccountID, RemoteActor: input.RemoteActor, RemoteInbox: input.RemoteInbox, ActivityID: input.ActivityID, Direction: direction}
	f.followers = append(f.followers, follow)
	return &follow, nil
}
func (f *fakeFollowsRepo) AcceptFollow(ctx context.Context, tx *db.Tx, followID string) error {
	return nil
}
func (f *fakeFollowsRepo) AcceptFollowByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor string) (*models.Follow, error) {
	return f.GetFollowByActor(ctx, tx, localAccountID, remoteActor, "follower")
}
func (f *fakeFollowsRepo) GetFollowByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor, direction string) (*models.Follow, error) {
	for _, follow := range f.followers {
		if follow.LocalAccountID == localAccountID && follow.RemoteActor == remoteActor && follow.Direction == direction {
			return &follow, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (f *fakeFollowsRepo) CreateFollowing(ctx context.Context, tx *db.Tx, input repos.CreateFollowInput) (*models.Follow, error) {
	input.Direction = "following"
	return f.CreateFollow(ctx, tx, input)
}
func (f *fakeFollowsRepo) AcceptFollowingByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor string) error {
	return nil
}
func (f *fakeFollowsRepo) RejectFollowingByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor string) error {
	return nil
}
func (f *fakeFollowsRepo) DeleteFollowingByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor string) error {
	return f.deleteByActor(localAccountID, remoteActor, "following")
}
func (f *fakeFollowsRepo) DeleteFollowByActor(ctx context.Context, tx *db.Tx, localAccountID, remoteActor string) error {
	return f.deleteByActor(localAccountID, remoteActor, "follower")
}
func (f *fakeFollowsRepo) deleteByActor(localAccountID, remoteActor, direction string) error {
	for i, follower := range f.followers {
		if follower.LocalAccountID == localAccountID && follower.RemoteActor == remoteActor && follower.Direction == direction {
			f.followers = append(f.followers[:i], f.followers[i+1:]...)
			return nil
		}
	}
	return nil
}
func (f *fakeFollowsRepo) ListFollowers(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Follow, error) {
	return f.followers, nil
}
func (f *fakeFollowsRepo) ListFollowersPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit, offset int) ([]models.Follow, error) {
	return f.followers, nil
}
func (f *fakeFollowsRepo) ListPendingFollowers(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Follow, error) {
	res := []models.Follow{}
	for _, follow := range f.followers {
		if follow.Direction == "follower" && follow.AcceptedAt == nil {
			res = append(res, follow)
		}
	}
	return res, nil
}
func (f *fakeFollowsRepo) CountFollowers(ctx context.Context, tx *db.Tx, localAccountID string) (int, error) {
	return len(f.followers), nil
}
func (f *fakeFollowsRepo) ListFollowing(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Follow, error) {
	return f.ListFollowingIncludingPending(ctx, tx, localAccountID)
}
func (f *fakeFollowsRepo) ListFollowingIncludingPending(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Follow, error) {
	res := []models.Follow{}
	for _, follow := range f.followers {
		if follow.Direction == "following" {
			res = append(res, follow)
		}
	}
	return res, nil
}

func (f *fakeFollowsRepo) ListLocalFollowersOfRemoteActor(ctx context.Context, tx *db.Tx, remoteActor string) ([]models.Follow, error) {
	res := []models.Follow{}
	for _, follow := range f.followers {
		if follow.RemoteActor == remoteActor && follow.Direction == "following" && follow.AcceptedAt != nil {
			res = append(res, follow)
		}
	}
	return res, nil
}

type fakeDomainBlocksRepo struct{}

func (fakeDomainBlocksRepo) CreateDomainBlock(ctx context.Context, tx *db.Tx, input repos.CreateDomainBlockInput) (*models.DomainBlock, error) {
	return &models.DomainBlock{Domain: input.Domain, Severity: models.DomainBlockSeveritySuspend}, nil
}
func (fakeDomainBlocksRepo) DeleteDomainBlock(ctx context.Context, tx *db.Tx, domain string) error {
	return nil
}
func (fakeDomainBlocksRepo) ListDomainBlocks(ctx context.Context, tx *db.Tx) ([]models.DomainBlock, error) {
	return nil, nil
}
func (fakeDomainBlocksRepo) GetDomainBlock(ctx context.Context, tx *db.Tx, domain string) (*models.DomainBlock, error) {
	return &models.DomainBlock{Domain: domain}, nil
}
func (fakeDomainBlocksRepo) DomainIsSuspended(ctx context.Context, tx *db.Tx, domain string) (bool, error) {
	return false, nil
}

type fakeSocialRepo struct{}

func (fakeSocialRepo) CreateInteraction(ctx context.Context, tx *db.Tx, localAccountID, noteID, typ string) (*models.StatusInteraction, error) {
	return &models.StatusInteraction{LocalAccountID: localAccountID, NoteID: noteID, Type: typ}, nil
}
func (fakeSocialRepo) DeleteInteraction(ctx context.Context, tx *db.Tx, localAccountID, noteID, typ string) error {
	return nil
}
func (fakeSocialRepo) InteractionExists(ctx context.Context, tx *db.Tx, localAccountID, noteID, typ string) (bool, error) {
	return false, nil
}
func (fakeSocialRepo) ListInteractions(ctx context.Context, tx *db.Tx, localAccountID, typ string, limit int) ([]models.StatusInteraction, error) {
	return nil, nil
}
func (fakeSocialRepo) CreateNotification(ctx context.Context, tx *db.Tx, localAccountID, actorAccountID, typ string, statusID *string) (*models.Notification, error) {
	return &models.Notification{}, nil
}
func (fakeSocialRepo) ListNotifications(ctx context.Context, tx *db.Tx, localAccountID string, limit int) ([]models.Notification, error) {
	return nil, nil
}
func (fakeSocialRepo) DeleteNotification(ctx context.Context, tx *db.Tx, localAccountID, notificationID string) error {
	return nil
}
func (fakeSocialRepo) DeleteNotificationsByActorAndType(ctx context.Context, tx *db.Tx, localAccountID, actorAccountID, typ string) error {
	return nil
}
func (fakeSocialRepo) ClearNotifications(ctx context.Context, tx *db.Tx, localAccountID string) error {
	return nil
}

type trackingSocialRepo struct {
	deleted []string
}

func (trackingSocialRepo) CreateInteraction(ctx context.Context, tx *db.Tx, localAccountID, noteID, typ string) (*models.StatusInteraction, error) {
	return &models.StatusInteraction{LocalAccountID: localAccountID, NoteID: noteID, Type: typ}, nil
}
func (trackingSocialRepo) DeleteInteraction(ctx context.Context, tx *db.Tx, localAccountID, noteID, typ string) error {
	return nil
}
func (trackingSocialRepo) InteractionExists(ctx context.Context, tx *db.Tx, localAccountID, noteID, typ string) (bool, error) {
	return false, nil
}
func (trackingSocialRepo) ListInteractions(ctx context.Context, tx *db.Tx, localAccountID, typ string, limit int) ([]models.StatusInteraction, error) {
	return nil, nil
}
func (trackingSocialRepo) CreateNotification(ctx context.Context, tx *db.Tx, localAccountID, actorAccountID, typ string, statusID *string) (*models.Notification, error) {
	return &models.Notification{}, nil
}
func (trackingSocialRepo) ListNotifications(ctx context.Context, tx *db.Tx, localAccountID string, limit int) ([]models.Notification, error) {
	return nil, nil
}
func (trackingSocialRepo) DeleteNotification(ctx context.Context, tx *db.Tx, localAccountID, notificationID string) error {
	return nil
}
func (t *trackingSocialRepo) DeleteNotificationsByActorAndType(ctx context.Context, tx *db.Tx, localAccountID, actorAccountID, typ string) error {
	t.deleted = append(t.deleted, localAccountID+"|"+actorAccountID+"|"+typ)
	return nil
}
func (trackingSocialRepo) ClearNotifications(ctx context.Context, tx *db.Tx, localAccountID string) error {
	return nil
}

type fakePollsRepo struct{}

func (fakePollsRepo) CreatePoll(ctx context.Context, tx *db.Tx, input repos.CreatePollInput) ([]models.PollOption, error) {
	return fakePollOptions(input.NoteID, input.Options), nil
}
func (fakePollsRepo) ReplacePoll(ctx context.Context, tx *db.Tx, input repos.CreatePollInput) ([]models.PollOption, error) {
	return fakePollOptions(input.NoteID, input.Options), nil
}
func (fakePollsRepo) GetPollOptions(ctx context.Context, tx *db.Tx, noteID string) ([]models.PollOption, error) {
	return nil, nil
}
func (fakePollsRepo) CreateLocalVote(ctx context.Context, tx *db.Tx, noteID, localAccountID string, choices []int, multiple bool) ([]models.PollOption, error) {
	return nil, nil
}
func (fakePollsRepo) CreateRemoteVote(ctx context.Context, tx *db.Tx, noteID, remoteActor, optionTitle string, multiple bool) ([]models.PollOption, error) {
	return nil, nil
}
func (fakePollsRepo) LocalVoteChoices(ctx context.Context, tx *db.Tx, noteID, localAccountID string) ([]int, error) {
	return nil, nil
}

func fakePollOptions(noteID string, titles []string) []models.PollOption {
	options := make([]models.PollOption, 0, len(titles))
	for i, title := range titles {
		options = append(options, models.PollOption{ID: "poll-option", NoteID: noteID, Title: title, Position: i})
	}
	return options
}

type fakeBoostsRepo struct {
	created []repos.CreateBoostInput
	deleted []string
}

func (f *fakeBoostsRepo) CreateBoost(ctx context.Context, tx *db.Tx, input repos.CreateBoostInput) (*models.Boost, error) {
	f.created = append(f.created, input)
	return &models.Boost{LocalAccountID: input.LocalAccountID, Actor: input.Actor, NoteID: input.NoteID, URI: input.URI, PublishedAt: input.PublishedAt}, nil
}
func (f *fakeBoostsRepo) DeleteBoost(ctx context.Context, tx *db.Tx, localAccountID, actor, noteID string) error {
	f.deleted = append(f.deleted, localAccountID+"|"+actor+"|"+noteID)
	return nil
}
func (fakeBoostsRepo) ListTimelineBoosts(ctx context.Context, tx *db.Tx, localAccountID string, limit int, maxID string) ([]models.Boost, error) {
	return nil, nil
}
func (fakeBoostsRepo) ListActorBoosts(ctx context.Context, tx *db.Tx, localAccountID, actor string, limit int, maxID string) ([]models.Boost, error) {
	return nil, nil
}
func (fakeBoostsRepo) CountBoostsForNote(ctx context.Context, tx *db.Tx, noteID string) (int, error) {
	return 0, nil
}
func (fakeBoostsRepo) BoostExists(ctx context.Context, tx *db.Tx, localAccountID, actor, noteID string) (bool, error) {
	return false, nil
}

type fakeTx struct{}

func (fakeTx) NewInsert() any { return nil }
func (fakeTx) NewSelect() any { return nil }
func (fakeTx) NewUpdate() any { return nil }
func (fakeTx) NewDelete() any { return nil }

type fakeTxProvider struct{}

func (fakeTxProvider) RunInTx(ctx context.Context, options interface{}, runIn func(ctx context.Context, tx db.Tx) error) error {
	return runIn(ctx, fakeTx{})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func newTestHandler(accounts repos.AccountsRepo, activities repos.ActivitiesRepository, follows repos.FollowsRepository) *Handler {
	return NewHandler(HandlerConfig{
		TxProvider:       fakeTxProvider{},
		AccountsRepo:     accounts,
		ActivitiesRepo:   activities,
		FollowsRepo:      follows,
		NotesRepo:        &fakeNotesRepo{},
		SocialRepo:       fakeSocialRepo{},
		BoostsRepo:       &fakeBoostsRepo{},
		PollsRepo:        fakePollsRepo{},
		DomainBlocksRepo: fakeDomainBlocksRepo{},
		DeliveryJobsRepo: fakeDeliveryJobsRepo{},
		FetchJobsRepo:    fakeFetchJobsRepo{},
		Serializer:       apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer: adapters.NewContentSanitizer(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})},
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
}

func TestUserProfileHandlerReturnsActivityPubContentType(t *testing.T) {
	app := fiber.New()
	newTestHandler(fakeAccountsRepo{}, &fakeActivitiesRepo{}, &fakeFollowsRepo{}).SetupRoutes(app)

	resp, err := app.Test(httptest.NewRequest("GET", "/users/alice", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get(fiber.HeaderContentType); !strings.HasPrefix(ct, "application/activity+json") {
		t.Fatalf("expected activitypub content type, got %q", ct)
	}
}

func TestUserProfileHandlerReturnsEmptyOutboxCollection(t *testing.T) {
	app := fiber.New()
	newTestHandler(fakeAccountsRepo{}, &fakeActivitiesRepo{}, &fakeFollowsRepo{}).SetupRoutes(app)

	resp, err := app.Test(httptest.NewRequest("GET", "/users/alice/outbox", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get(fiber.HeaderContentType); !strings.HasPrefix(ct, "application/activity+json") {
		t.Fatalf("expected activitypub content type, got %q", ct)
	}

	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if got["type"] != "OrderedCollection" {
		t.Fatalf("expected OrderedCollection, got %v", got["type"])
	}
	if got["totalItems"] != float64(0) {
		t.Fatalf("expected empty collection, got totalItems=%v", got["totalItems"])
	}
}

func TestUserProfileHandlerDereferencesPublicObject(t *testing.T) {
	app := fiber.New()
	notes := &fakeNotesRepo{notes: []models.Note{{ID: "note-1", LocalAccountID: "account-1", URI: "https://example.org/users/alice/objects/object-1", Content: "<p>Hello</p>", Visibility: "public", AttributedTo: "https://example.org/users/alice", PublishedAt: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)}}}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		SocialRepo:         fakeSocialRepo{},
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	resp, err := app.Test(httptest.NewRequest("GET", "/users/alice/objects/object-1", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if got["type"] != "Note" || got["id"] != "https://example.org/users/alice/objects/object-1" {
		t.Fatalf("unexpected object document: %#v", got)
	}
}

func TestUserProfileHandlerDoesNotDereferencePrivateObject(t *testing.T) {
	app := fiber.New()
	notes := &fakeNotesRepo{notes: []models.Note{{ID: "note-1", LocalAccountID: "account-1", URI: "https://example.org/users/alice/objects/object-1", Content: "secret", Visibility: "direct", AttributedTo: "https://example.org/users/alice"}}}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		SocialRepo:         fakeSocialRepo{},
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	resp, err := app.Test(httptest.NewRequest("GET", "/users/alice/objects/object-1", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUserProfileHandlerDereferencesFollowersOnlyObjectForSignedAcceptedFollower(t *testing.T) {
	app := fiber.New()
	acceptedAt := time.Now().UTC()
	remoteActor := "https://remote.example/users/bob"
	notes := &fakeNotesRepo{notes: []models.Note{{ID: "note-1", LocalAccountID: "account-1", URI: "https://example.org/users/alice/objects/object-1", Content: "followers only", Visibility: "private", AttributedTo: "https://example.org/users/alice", PublishedAt: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)}}}
	follows := &fakeFollowsRepo{followers: []models.Follow{{ID: "follow-1", LocalAccountID: "account-1", RemoteActor: remoteActor, Direction: "follower", AcceptedAt: &acceptedAt}}}
	verifier := &acceptingSignatureVerifier{}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        follows,
		NotesRepo:          notes,
		SocialRepo:         fakeSocialRepo{},
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		SignatureVerifier:  verifier,
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	req := httptest.NewRequest("GET", "/users/alice/objects/object-1", nil)
	req.Header.Set("Signature", `keyId="https://remote.example/users/bob#main-key"`)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(verifier.actors) != 1 || verifier.actors[0] != remoteActor {
		t.Fatalf("expected verifier to receive requester actor, got %#v", verifier.actors)
	}
}

func TestUserProfileHandlerDereferencesPublicActivity(t *testing.T) {
	app := fiber.New()
	activities := &fakeActivitiesRepo{activities: []models.Activity{{LocalAccountID: "account-1", Direction: models.ActivityDirectionOutbox, Type: "Create", Object: "https://example.org/users/alice/objects/object-1", RawJSON: `{"@context":"https://www.w3.org/ns/activitystreams","id":"https://example.org/users/alice/activities/activity-1","type":"Create","actor":"https://example.org/users/alice","object":"https://example.org/users/alice/objects/object-1"}`}}}
	notes := &fakeNotesRepo{notes: []models.Note{{ID: "note-1", LocalAccountID: "account-1", URI: "https://example.org/users/alice/objects/object-1", Content: "hello", Visibility: "public", AttributedTo: "https://example.org/users/alice"}}}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     activities,
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		SocialRepo:         fakeSocialRepo{},
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	resp, err := app.Test(httptest.NewRequest("GET", "/users/alice/activities/activity-1", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUserProfileHandlerAcceptsInboxActivities(t *testing.T) {
	app := fiber.New()
	newTestHandler(fakeAccountsRepo{}, &fakeActivitiesRepo{}, &fakeFollowsRepo{}).SetupRoutes(app)

	body := `{"type":"Create","actor":"https://remote.example/users/bob","object":"https://remote.example/notes/1"}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
}

func TestSharedInboxDispatchesToAddressedLocalActor(t *testing.T) {
	app := fiber.New()
	notes := &fakeNotesRepo{}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		SocialRepo:         fakeSocialRepo{},
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	body := `{"type":"Create","actor":"https://remote.example/users/bob","to":["https://example.org/users/alice"],"object":{"id":"https://remote.example/notes/1","type":"Note","content":"<p>shared</p>","attributedTo":"https://remote.example/users/bob","published":"2026-05-19T12:00:00Z"}}`
	resp, err := app.Test(httptest.NewRequest("POST", "/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if len(notes.notes) != 1 || notes.notes[0].PlainText != "shared" {
		t.Fatalf("expected shared inbox note to be stored, got %#v", notes.notes)
	}
}

func TestUserProfileHandlerStoresInboundCreateNote(t *testing.T) {
	app := fiber.New()
	notes := &fakeNotesRepo{}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		SocialRepo:         fakeSocialRepo{},
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	body := `{"type":"Create","actor":"https://remote.example/users/bob","object":{"id":"https://remote.example/notes/1","type":"Note","content":"<p>remote</p>","attributedTo":"https://remote.example/users/bob","published":"2026-05-19T12:00:00Z"}}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if len(notes.notes) != 1 || notes.notes[0].PlainText != "remote" {
		t.Fatalf("expected stored remote note, got %#v", notes.notes)
	}
}

func TestUserProfileHandlerStoresInboundArticleAsNoteLikeObject(t *testing.T) {
	app := fiber.New()
	notes := &fakeNotesRepo{}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		SocialRepo:         fakeSocialRepo{},
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	body := `{"type":"Create","actor":"https://remote.example/users/bob","object":{"id":"https://remote.example/articles/1","type":"Article","name":"Article title","attributedTo":"https://remote.example/users/bob","published":"2026-05-19T12:00:00Z"}}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if len(notes.notes) != 1 || notes.notes[0].PlainText != "Article title" {
		t.Fatalf("expected stored article fallback text, got %#v", notes.notes)
	}
}

func TestUserProfileHandlerUndoLikeDeletesNotification(t *testing.T) {
	app := fiber.New()
	social := &trackingSocialRepo{}
	notes := &fakeNotesRepo{notes: []models.Note{{ID: "note-1", LocalAccountID: "account-1", URI: "https://example.org/users/alice/objects/note-1", Visibility: "public", AttributedTo: "https://example.org/users/alice"}}}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		SocialRepo:         social,
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	body := `{"type":"Undo","actor":"https://remote.example/users/bob","object":{"type":"Like","actor":"https://remote.example/users/bob","object":"https://example.org/users/alice/objects/note-1"}}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if len(social.deleted) != 1 || social.deleted[0] != "account-1|https://remote.example/users/bob|favourite" {
		t.Fatalf("expected favourite notification deletion, got %#v", social.deleted)
	}
}

func TestUserProfileHandlerUndoAnnounceDeletesBoostAndNotification(t *testing.T) {
	app := fiber.New()
	social := &trackingSocialRepo{}
	boosts := &fakeBoostsRepo{}
	notes := &fakeNotesRepo{notes: []models.Note{{ID: "note-1", LocalAccountID: "account-1", URI: "https://example.org/users/alice/objects/note-1", Visibility: "public", AttributedTo: "https://example.org/users/alice"}}}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		SocialRepo:         social,
		BoostsRepo:         boosts,
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	body := `{"type":"Undo","actor":"https://remote.example/users/bob","object":{"type":"Announce","actor":"https://remote.example/users/bob","object":"https://example.org/users/alice/objects/note-1"}}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if len(boosts.deleted) != 1 || boosts.deleted[0] != "account-1|https://remote.example/users/bob|note-1" {
		t.Fatalf("expected boost deletion, got %#v", boosts.deleted)
	}
	if len(social.deleted) != 1 || social.deleted[0] != "account-1|https://remote.example/users/bob|reblog" {
		t.Fatalf("expected reblog notification deletion, got %#v", social.deleted)
	}
}

func TestUserProfileHandlerDeletesNoteOnInboundTombstoneUpdate(t *testing.T) {
	app := fiber.New()
	notes := &fakeNotesRepo{notes: []models.Note{{ID: "note-1", LocalAccountID: "account-1", URI: "https://remote.example/notes/1", Content: "gone", Visibility: "public", AttributedTo: "https://remote.example/users/bob"}}}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		SocialRepo:         fakeSocialRepo{},
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	body := `{"type":"Update","actor":"https://remote.example/users/bob","object":{"id":"https://remote.example/notes/1","type":"Tombstone","formerType":"Note"}}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if len(notes.notes) != 0 {
		t.Fatalf("expected tombstoned note to be deleted, got %#v", notes.notes)
	}
}

func TestUserProfileHandlerRemoteBlockRemovesRelationships(t *testing.T) {
	app := fiber.New()
	remoteActor := "https://remote.example/users/bob"
	acceptedAt := time.Now().UTC()
	follows := &fakeFollowsRepo{followers: []models.Follow{
		{ID: "follower", LocalAccountID: "account-1", RemoteActor: remoteActor, Direction: "follower", AcceptedAt: &acceptedAt},
		{ID: "following", LocalAccountID: "account-1", RemoteActor: remoteActor, Direction: "following", AcceptedAt: &acceptedAt},
	}}
	newTestHandler(fakeAccountsRepo{}, &fakeActivitiesRepo{}, follows).SetupRoutes(app)

	body := `{"type":"Block","actor":"https://remote.example/users/bob","object":"https://example.org/users/alice"}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if len(follows.followers) != 0 {
		t.Fatalf("expected block to remove follower/following relationships, got %#v", follows.followers)
	}
}

func TestUserProfileHandlerAcceptsFlagWithoutSideEffects(t *testing.T) {
	app := fiber.New()
	activities := &fakeActivitiesRepo{}
	notes := &fakeNotesRepo{}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     activities,
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		SocialRepo:         fakeSocialRepo{},
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	body := `{"type":"Flag","actor":"https://remote.example/users/bob","object":"https://example.org/users/alice/objects/note-1","content":"report"}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if len(activities.activities) != 1 || activities.activities[0].Type != "Flag" {
		t.Fatalf("expected raw Flag activity to be stored, got %#v", activities.activities)
	}
	if len(notes.notes) != 0 {
		t.Fatalf("expected Flag not to create/delete notes, got %#v", notes.notes)
	}
}

func TestUserProfileHandlerMoveFetchesTargetButDoesNotRewriteRelationships(t *testing.T) {
	app := fiber.New()
	remoteActor := "https://remote.example/users/bob"
	acceptedAt := time.Now().UTC()
	follows := &fakeFollowsRepo{followers: []models.Follow{{ID: "following", LocalAccountID: "account-1", RemoteActor: remoteActor, Direction: "following", AcceptedAt: &acceptedAt}}}
	fetcher := &fakeActorFetcher{}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        follows,
		NotesRepo:          &fakeNotesRepo{},
		SocialRepo:         fakeSocialRepo{},
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		ActorFetcher:       fetcher,
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	body := `{"type":"Move","actor":"https://remote.example/users/bob","object":"https://remote.example/users/bob","target":"https://new.example/users/bob"}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if len(fetcher.fetched) != 1 || fetcher.fetched[0] != "https://new.example/users/bob" {
		t.Fatalf("expected Move target to be fetched, got %#v", fetcher.fetched)
	}
	if len(follows.followers) != 1 || follows.followers[0].RemoteActor != remoteActor {
		t.Fatalf("expected Move not to rewrite relationships, got %#v", follows.followers)
	}
}

func TestUserProfileHandlerRejectsForgedInboundCreateAuthor(t *testing.T) {
	app := fiber.New()
	notes := &fakeNotesRepo{}
	handler := NewHandler(HandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		SocialRepo:         fakeSocialRepo{},
		BoostsRepo:         &fakeBoostsRepo{},
		PollsRepo:          fakePollsRepo{},
		DomainBlocksRepo:   fakeDomainBlocksRepo{},
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		FetchJobsRepo:      fakeFetchJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
		Host:               "https://example.org",
	})
	handler.SetupRoutes(app)

	body := `{"type":"Create","actor":"https://remote.example/users/bob","object":{"id":"https://remote.example/notes/1","type":"Note","content":"forged","attributedTo":"https://remote.example/users/mallory","published":"2026-05-19T12:00:00Z"}}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	if len(notes.notes) != 0 {
		t.Fatalf("expected forged note to be rejected, got %#v", notes.notes)
	}
}

func TestUserProfileHandlerStoresAndAcceptsFollow(t *testing.T) {
	app := fiber.New()
	follows := &fakeFollowsRepo{}
	newTestHandler(fakeAccountsRepo{}, &fakeActivitiesRepo{}, follows).SetupRoutes(app)

	body := `{"type":"Follow","actor":{"id":"https://remote.example/users/bob","inbox":"https://remote.example/users/bob/inbox"},"object":"https://example.org/users/alice"}`
	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/inbox", strings.NewReader(body)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	if len(follows.followers) != 1 {
		t.Fatalf("expected one follower, got %d", len(follows.followers))
	}
}

func TestUserProfileHandlerReturnsNotFoundForMissingActor(t *testing.T) {
	app := fiber.New()
	newTestHandler(fakeAccountsRepo{err: sql.ErrNoRows}, &fakeActivitiesRepo{}, &fakeFollowsRepo{}).SetupRoutes(app)

	resp, err := app.Test(httptest.NewRequest("GET", "/users/missing", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
