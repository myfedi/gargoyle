package repos

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
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
	URI                   *string
	URL                   *string // optional
	InboxURI              *string
	OutboxURI             *string
	FollowingURI          *string
	FollowersURI          *string
	FeaturedCollectionURI *string
	PrivateKey            *string // nullable
	PublicKey             string
	ActorType             models.ActorType // maps to enum
}

type AccountsRepo interface {
	CreateAccount(input CreateAccountInput) (*models.Account, error)
	GetAccountByUserID(userID string) (*models.Account, error)
}
