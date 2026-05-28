package users

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
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type fakeDeliveryJobsRepo struct{}

func (f fakeDeliveryJobsRepo) CreateDeliveryJob(ctx context.Context, tx *db.Tx, input repos.CreateDeliveryJobInput) (*models.DeliveryJob, error) {
	return &models.DeliveryJob{ID: "job-1", AccountID: input.AccountID, InboxURL: input.InboxURL, Payload: input.Payload, NextAttemptAt: input.NextAttemptAt}, nil
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

type fakeAccountsRepo struct{ err error }

func (f fakeAccountsRepo) CreateAccount(ctx context.Context, tx *db.Tx, input repos.CreateAccountInput) (*models.Account, error) {
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
func (f *fakeActivitiesRepo) ListOutboxActivitiesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit int, offset int) ([]models.Activity, error) {
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
func (f *fakeActivitiesRepo) ListPublicOutboxActivitiesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit int, offset int) ([]models.Activity, error) {
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
	note := models.Note{ID: "note-1", LocalAccountID: input.LocalAccountID, ActivityID: input.ActivityID, URI: input.URI, Content: input.Content, PlainText: input.PlainText, Visibility: input.Visibility, Sensitive: input.Sensitive, SpoilerText: input.SpoilerText, AttributedTo: input.AttributedTo, InReplyToID: input.InReplyToID, InReplyToURI: input.InReplyToURI, PublishedAt: input.PublishedAt}
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

func (f *fakeNotesRepo) UpdateNoteByURI(ctx context.Context, tx *db.Tx, uri string, content string, plainText string) error {
	for i := range f.notes {
		if f.notes[i].URI == uri {
			f.notes[i].Content = content
			f.notes[i].PlainText = plainText
		}
	}
	return nil
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
func (f *fakeNotesRepo) ListDirectNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListKnownPublicTimelineNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListKnownLocalTimelineNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, localActorPrefix string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListKnownRemoteTimelineNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, localActorPrefix string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListAttributedNotesPaged(ctx context.Context, tx *db.Tx, localAccountID string, attributedTo string, limit int, maxID string) ([]models.Note, error) {
	return f.notes, nil
}
func (f *fakeNotesRepo) ListReplies(ctx context.Context, tx *db.Tx, localAccountID string, parentID string) ([]models.Note, error) {
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
func (f *fakeFollowsRepo) CreateFollowing(ctx context.Context, tx *db.Tx, input repos.CreateFollowInput) (*models.Follow, error) {
	input.Direction = "following"
	return f.CreateFollow(ctx, tx, input)
}
func (f *fakeFollowsRepo) AcceptFollowingByActor(ctx context.Context, tx *db.Tx, localAccountID string, remoteActor string) error {
	return nil
}
func (f *fakeFollowsRepo) RejectFollowingByActor(ctx context.Context, tx *db.Tx, localAccountID string, remoteActor string) error {
	return nil
}
func (f *fakeFollowsRepo) DeleteFollowingByActor(ctx context.Context, tx *db.Tx, localAccountID string, remoteActor string) error {
	return nil
}
func (f *fakeFollowsRepo) DeleteFollowByActor(ctx context.Context, tx *db.Tx, localAccountID string, remoteActor string) error {
	for i, follower := range f.followers {
		if follower.LocalAccountID == localAccountID && follower.RemoteActor == remoteActor {
			f.followers = append(f.followers[:i], f.followers[i+1:]...)
			return nil
		}
	}
	return nil
}
func (f *fakeFollowsRepo) ListFollowers(ctx context.Context, tx *db.Tx, localAccountID string) ([]models.Follow, error) {
	return f.followers, nil
}
func (f *fakeFollowsRepo) ListFollowersPaged(ctx context.Context, tx *db.Tx, localAccountID string, limit int, offset int) ([]models.Follow, error) {
	return f.followers, nil
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

func newTestHandler(accounts repos.AccountsRepo, activities repos.ActivitiesRepository, follows repos.FollowsRepository) *UsersWebHandler {
	return NewUsersWebHandler(UsersWebHandlerConfig{
		TxProvider:       fakeTxProvider{},
		AccountsRepo:     accounts,
		ActivitiesRepo:   activities,
		FollowsRepo:      follows,
		NotesRepo:        &fakeNotesRepo{},
		DeliveryJobsRepo: fakeDeliveryJobsRepo{},
		Serializer:       apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer: adapters.NewContentSanitizer(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})},
		BodyLimitBytes:     1 << 20,
		DeliveryQueueSize:  16,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
	})
}

func TestUserProfileHandlerReturnsActivityPubContentType(t *testing.T) {
	app := fiber.New()
	newTestHandler(fakeAccountsRepo{}, &fakeActivitiesRepo{}, &fakeFollowsRepo{}).SetupUserProfileHandler(app)

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
	newTestHandler(fakeAccountsRepo{}, &fakeActivitiesRepo{}, &fakeFollowsRepo{}).SetupUserProfileHandler(app)

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

func TestUserProfileHandlerAcceptsInboxActivities(t *testing.T) {
	app := fiber.New()
	newTestHandler(fakeAccountsRepo{}, &fakeActivitiesRepo{}, &fakeFollowsRepo{}).SetupUserProfileHandler(app)

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

func TestUserProfileHandlerCreatesNoteFromOutboxPost(t *testing.T) {
	app := fiber.New()
	notes := &fakeNotesRepo{}
	handler := NewUsersWebHandler(UsersWebHandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		DeliveryQueueSize:  16,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
	})
	handler.SetupUserProfileHandler(app)

	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/outbox", strings.NewReader(`{"content":"<p>hello <b>world</b></p>"}`)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if len(notes.notes) != 1 {
		t.Fatalf("expected one note, got %d", len(notes.notes))
	}
	if notes.notes[0].PlainText != "hello world" {
		t.Fatalf("expected stripped plaintext, got %q", notes.notes[0].PlainText)
	}
}

func TestUserProfileHandlerCreatesFollowing(t *testing.T) {
	app := fiber.New()
	follows := &fakeFollowsRepo{}
	newTestHandler(fakeAccountsRepo{}, &fakeActivitiesRepo{}, follows).SetupUserProfileHandler(app)

	resp, err := app.Test(httptest.NewRequest("POST", "/users/alice/following", strings.NewReader(`{"actor":"https://remote.example/users/bob","inbox":"https://remote.example/inbox"}`)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	following, err := follows.ListFollowing(context.Background(), nil, "account-1")
	if err != nil {
		t.Fatalf("ListFollowing returned error: %v", err)
	}
	if len(following) != 1 || following[0].RemoteActor != "https://remote.example/users/bob" {
		t.Fatalf("expected following bob, got %#v", following)
	}
}

func TestUserProfileHandlerStoresInboundCreateNote(t *testing.T) {
	app := fiber.New()
	notes := &fakeNotesRepo{}
	handler := NewUsersWebHandler(UsersWebHandlerConfig{
		TxProvider:         fakeTxProvider{},
		AccountsRepo:       fakeAccountsRepo{},
		ActivitiesRepo:     &fakeActivitiesRepo{},
		FollowsRepo:        &fakeFollowsRepo{},
		NotesRepo:          notes,
		DeliveryJobsRepo:   fakeDeliveryJobsRepo{},
		Serializer:         apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		ContentSanitizer:   adapters.NewContentSanitizer(),
		BodyLimitBytes:     1 << 20,
		DeliveryQueueSize:  16,
		AllowUnsignedInbox: true,
		DeliveryRetries:    1,
	})
	handler.SetupUserProfileHandler(app)

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

func TestUserProfileHandlerStoresAndAcceptsFollow(t *testing.T) {
	app := fiber.New()
	follows := &fakeFollowsRepo{}
	newTestHandler(fakeAccountsRepo{}, &fakeActivitiesRepo{}, follows).SetupUserProfileHandler(app)

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
	newTestHandler(fakeAccountsRepo{err: sql.ErrNoRows}, &fakeActivitiesRepo{}, &fakeFollowsRepo{}).SetupUserProfileHandler(app)

	resp, err := app.Test(httptest.NewRequest("GET", "/users/missing", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
