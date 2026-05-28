package activitypub

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/myfedi/gargoyle/domain/models"
	domainerrors "github.com/myfedi/gargoyle/domain/models/domainerrors"
	"github.com/myfedi/gargoyle/domain/ports/db"
	"github.com/myfedi/gargoyle/domain/ports/repos"
)

type fakeAccountsRepo struct {
	account *models.Account
	err     error
}

type fakeActorSerializer struct{}

func (f fakeActorSerializer) Marshall(account models.Account) (string, error) {
	return `{"type":"Person"}`, nil
}

func (f fakeActorSerializer) Unmarshall(input string) (*models.Account, error) {
	return nil, nil
}

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
	return f.account, nil
}

func (f fakeAccountsRepo) SearchLocalAccounts(ctx context.Context, tx *db.Tx, query string, limit int) ([]models.Account, error) {
	if f.account == nil {
		return nil, nil
	}
	return []models.Account{*f.account}, nil
}

func (f fakeAccountsRepo) AccountWithUsernameExists(ctx context.Context, tx *db.Tx, username string) (bool, error) {
	return false, nil
}

func TestGetUserProfileMapsMissingAccountToNotFound(t *testing.T) {
	uc := NewGetUserProfileUseCase(GetUserProfileUseCaseConfig{
		Serializer:   fakeActorSerializer{},
		AccountsRepo: fakeAccountsRepo{err: sql.ErrNoRows},
	})

	_, derr := uc.GetUserProfile(context.Background(), "missing")
	if derr == nil {
		t.Fatal("expected domain error")
	}
	if !errors.Is(derr.Code, domainerrors.ErrNotFound) {
		t.Fatalf("expected not found, got %v", derr.Code)
	}
}

func TestGetUserProfileReturnsActivityPubActor(t *testing.T) {
	uc := NewGetUserProfileUseCase(GetUserProfileUseCaseConfig{
		Serializer: fakeActorSerializer{},
		AccountsRepo: fakeAccountsRepo{account: &models.Account{
			Username:     "alice",
			URI:          "https://example.org/users/alice",
			InboxURI:     "https://example.org/users/alice/inbox",
			FollowersURI: "https://example.org/users/alice/followers",
			FollowingURI: "https://example.org/users/alice/following",
			PublicKey:    "public-key-pem",
			ActorType:    models.ActorTypePerson,
		}},
	})

	profile, derr := uc.GetUserProfile(context.Background(), "alice")
	if derr != nil {
		t.Fatalf("unexpected domain error: %v", derr)
	}
	if profile == "" {
		t.Fatal("expected non-empty profile")
	}
}
