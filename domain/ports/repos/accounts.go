package repos

import (
	"context"
	"time"

	"github.com/myfedi/gargoyle/domain/models"
	"github.com/myfedi/gargoyle/domain/ports/db"
)

// The data necessary (or optional) to create a new account.
type CreateAccountInput struct {
	ID                    string    // ULID
	UserID                *string   // foreign key to users.id
	CreatedAt             time.Time // optional override
	UpdatedAt             time.Time // optional override
	FetchedAt             time.Time // optional
	Username              string
	Domain                *string // optional; empty if local user
	DisplayName           *string // optional
	Summary               *string // optional
	URI                   string
	URL                   *string // optional
	Fields                []models.AccountProfileField
	AvatarMediaID         *string // optional local media id
	HeaderMediaID         *string // optional local media id
	AvatarURL             *string // optional remote/external URL
	HeaderURL             *string // optional remote/external URL
	InboxURI              string
	OutboxURI             *string
	FollowingURI          string
	FollowersURI          string
	FeaturedCollectionURI string
	PrivateKey            *string // nullable
	PublicKey             string
	ActorType             models.ActorType // maps to enum
	Locked                bool
}

type UpdateAccountProfileInput struct {
	DisplayName   *string
	Summary       *string
	Fields        []models.AccountProfileField
	AvatarMediaID *string
	HeaderMediaID *string
	AvatarURL     *string
	HeaderURL     *string
	Locked        *bool
	UpdatedAt     time.Time // optional override
}

type AccountsRepo interface {
	CreateAccount(ctx context.Context, tx *db.Tx, input CreateAccountInput) (*models.Account, error)
	UpdateLocalAccountProfile(ctx context.Context, tx *db.Tx, id string, input UpdateAccountProfileInput) (*models.Account, error)
	GetAccountByID(ctx context.Context, tx *db.Tx, id string) (*models.Account, error)
	GetAccountByUserID(ctx context.Context, tx *db.Tx, userID string) (*models.Account, error)
	GetLocalAccountByUsername(ctx context.Context, tx *db.Tx, username string) (*models.Account, error)
	SearchLocalAccounts(ctx context.Context, tx *db.Tx, query string, limit int) ([]models.Account, error)
	AccountWithUsernameExists(ctx context.Context, tx *db.Tx, username string) (bool, error)
}
