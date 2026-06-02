package repos

import (
	"context"
	"database/sql"
	"testing"

	"github.com/myfedi/gargoyle/domain/models"
	portrepos "github.com/myfedi/gargoyle/domain/ports/repos"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestAccountsRepoCreateAndLoadPreservesActorType(t *testing.T) {
	db := newTestDB(t)
	repo := NewAccountsRepo(db)
	userID := "user-1"

	_, err := repo.CreateAccount(context.Background(), nil, portrepos.CreateAccountInput{
		UserID:                &userID,
		Username:              "alice",
		Domain:                strPtr("example.org"),
		URI:                   "https://example.org/users/alice",
		InboxURI:              "https://example.org/users/alice/inbox",
		FollowersURI:          "https://example.org/users/alice/followers",
		FollowingURI:          "https://example.org/users/alice/following",
		FeaturedCollectionURI: "https://example.org/users/alice/collections/featured",
		PublicKey:             "public-key-pem",
		ActorType:             models.ActorTypePerson,
	})
	if err != nil {
		t.Fatalf("CreateAccount returned error: %v", err)
	}

	account, err := repo.GetLocalAccountByUsername(context.Background(), nil, "alice")
	if err != nil {
		t.Fatalf("GetLocalAccountByUsername returned error: %v", err)
	}
	if account.ActorType != models.ActorTypePerson {
		t.Fatalf("expected actor type Person, got %v", account.ActorType)
	}

	updated, err := repo.UpdateLocalAccountProfile(context.Background(), nil, account.ID, portrepos.UpdateAccountProfileInput{DisplayName: strPtr("Alice"), Summary: strPtr("bio"), AvatarMediaID: strPtr("avatar-1"), HeaderMediaID: strPtr("header-1")})
	if err != nil {
		t.Fatalf("UpdateLocalAccountProfile returned error: %v", err)
	}
	if updated.DisplayName == nil || *updated.DisplayName != "Alice" || updated.AvatarMediaID == nil || *updated.AvatarMediaID != "avatar-1" {
		t.Fatalf("profile update was not persisted: %+v", updated)
	}
}

func newTestDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })

	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.ExecContext(context.Background(), `
CREATE TABLE accounts (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    user_id CHAR(26) NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fetched_at DATETIME,
    username TEXT NOT NULL,
    domain TEXT,
    display_name TEXT,
    summary TEXT,
    uri TEXT NOT NULL UNIQUE,
    url TEXT,
    avatar_media_id CHAR(26),
    header_media_id CHAR(26),
    avatar_url TEXT,
    header_url TEXT,
    inbox_uri TEXT,
    outbox_uri TEXT,
    following_uri TEXT,
    followers_uri TEXT,
    featured_collection_uri TEXT,
    private_key TEXT,
    public_key TEXT NOT NULL UNIQUE,
    actor_type INTEGER NOT NULL DEFAULT 0,
    UNIQUE(username, domain)
);`)
	if err != nil {
		t.Fatalf("create accounts table: %v", err)
	}

	return db
}

func strPtr(s string) *string { return &s }
