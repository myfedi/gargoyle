package users

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	apAdapters "github.com/myfedi/gargoyle/adapters/activitypub"
	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type fakeAccountsRepo struct{ err error }

func (f fakeAccountsRepo) CreateAccount(tx *db.Tx, input repos.CreateAccountInput) (*models.Account, error) {
	return nil, nil
}
func (f fakeAccountsRepo) GetAccountByUserID(tx *db.Tx, userID string) (*models.Account, error) {
	return nil, nil
}
func (f fakeAccountsRepo) GetLocalAccountByUsername(tx *db.Tx, username string) (*models.Account, error) {
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
func (f fakeAccountsRepo) AccountWithUsernameExists(tx *db.Tx, username string) (bool, error) {
	return false, nil
}

type fakeActivitiesRepo struct{ activities []models.Activity }

func (f *fakeActivitiesRepo) CreateActivity(tx *db.Tx, input repos.CreateActivityInput) (*models.Activity, error) {
	activity := models.Activity{ID: "activity-1", LocalAccountID: input.LocalAccountID, Direction: input.Direction, Type: input.Type, Actor: input.Actor, Object: input.Object, RawJSON: input.RawJSON}
	f.activities = append(f.activities, activity)
	return &activity, nil
}
func (f *fakeActivitiesRepo) ListOutboxActivities(tx *db.Tx, localAccountID string) ([]models.Activity, error) {
	return f.activities, nil
}
func (f *fakeActivitiesRepo) ListOutboxActivitiesPaged(tx *db.Tx, localAccountID string, limit int, offset int) ([]models.Activity, error) {
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
func (f *fakeActivitiesRepo) CountOutboxActivities(tx *db.Tx, localAccountID string) (int, error) {
	return len(f.activities), nil
}

type fakeNotesRepo struct{ notes []models.Note }

func (f *fakeNotesRepo) GetLocalPostsCount() (int, error) { return len(f.notes), nil }
func (f *fakeNotesRepo) CreateNote(tx *db.Tx, input repos.CreateNoteInput) (*models.Note, error) {
	note := models.Note{ID: "note-1", LocalAccountID: input.LocalAccountID, ActivityID: input.ActivityID, URI: input.URI, Content: input.Content, PlainText: input.PlainText, AttributedTo: input.AttributedTo, PublishedAt: input.PublishedAt}
	f.notes = append(f.notes, note)
	return &note, nil
}
func (f *fakeNotesRepo) UpdateNoteByURI(tx *db.Tx, uri string, content string, plainText string) error {
	for i := range f.notes {
		if f.notes[i].URI == uri {
			f.notes[i].Content = content
			f.notes[i].PlainText = plainText
		}
	}
	return nil
}
func (f *fakeNotesRepo) DeleteNoteByURI(tx *db.Tx, uri string) error {
	for i, note := range f.notes {
		if note.URI == uri {
			f.notes = append(f.notes[:i], f.notes[i+1:]...)
			return nil
		}
	}
	return nil
}
func (f *fakeNotesRepo) ListLocalNotes(tx *db.Tx, localAccountID string) ([]models.Note, error) {
	return f.notes, nil
}

type fakeFollowsRepo struct{ followers []models.Follow }

func (f *fakeFollowsRepo) CreateFollow(tx *db.Tx, input repos.CreateFollowInput) (*models.Follow, error) {
	direction := input.Direction
	if direction == "" {
		direction = "follower"
	}
	follow := models.Follow{ID: "follow-1", LocalAccountID: input.LocalAccountID, RemoteActor: input.RemoteActor, RemoteInbox: input.RemoteInbox, ActivityID: input.ActivityID, Direction: direction}
	f.followers = append(f.followers, follow)
	return &follow, nil
}
func (f *fakeFollowsRepo) AcceptFollow(tx *db.Tx, followID string) error { return nil }
func (f *fakeFollowsRepo) CreateFollowing(tx *db.Tx, input repos.CreateFollowInput) (*models.Follow, error) {
	input.Direction = "following"
	return f.CreateFollow(tx, input)
}
func (f *fakeFollowsRepo) AcceptFollowingByActor(tx *db.Tx, localAccountID string, remoteActor string) error {
	return nil
}
func (f *fakeFollowsRepo) RejectFollowingByActor(tx *db.Tx, localAccountID string, remoteActor string) error {
	return nil
}
func (f *fakeFollowsRepo) DeleteFollowByActor(tx *db.Tx, localAccountID string, remoteActor string) error {
	for i, follower := range f.followers {
		if follower.LocalAccountID == localAccountID && follower.RemoteActor == remoteActor {
			f.followers = append(f.followers[:i], f.followers[i+1:]...)
			return nil
		}
	}
	return nil
}
func (f *fakeFollowsRepo) ListFollowers(tx *db.Tx, localAccountID string) ([]models.Follow, error) {
	return f.followers, nil
}
func (f *fakeFollowsRepo) ListFollowersPaged(tx *db.Tx, localAccountID string, limit int, offset int) ([]models.Follow, error) {
	return f.followers, nil
}
func (f *fakeFollowsRepo) CountFollowers(tx *db.Tx, localAccountID string) (int, error) {
	return len(f.followers), nil
}
func (f *fakeFollowsRepo) ListFollowing(tx *db.Tx, localAccountID string) ([]models.Follow, error) {
	res := []models.Follow{}
	for _, follow := range f.followers {
		if follow.Direction == "following" {
			res = append(res, follow)
		}
	}
	return res, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func newTestHandler(accounts repos.AccountsRepo, activities repos.ActivitiesRepository, follows repos.FollowsRepository) *UsersWebHandler {
	return NewUsersWebHandler(UsersWebHandlerConfig{
		AccountsRepo:   accounts,
		ActivitiesRepo: activities,
		FollowsRepo:    follows,
		NotesRepo:      &fakeNotesRepo{},
		Serializer:     apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})},
		DeliveryRetries: 1,
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
		AccountsRepo:    fakeAccountsRepo{},
		ActivitiesRepo:  &fakeActivitiesRepo{},
		FollowsRepo:     &fakeFollowsRepo{},
		NotesRepo:       notes,
		Serializer:      apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		DeliveryRetries: 1,
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
	following, err := follows.ListFollowing(nil, "account-1")
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
		AccountsRepo:    fakeAccountsRepo{},
		ActivitiesRepo:  &fakeActivitiesRepo{},
		FollowsRepo:     &fakeFollowsRepo{},
		NotesRepo:       notes,
		Serializer:      apAdapters.NewActorSerializer(apAdapters.ActorSerializerConfig{}),
		DeliveryRetries: 1,
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
