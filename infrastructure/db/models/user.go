package models

import (
	"time"

	"github.com/myfedi/gargoyle/domain/models"
)

type User struct {
	ID           string    `bun:"type:CHAR(26),pk,nullzero,notnull,unique"`
	CreatedAt    time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	UpdatedAt    time.Time `bun:"type:timestamptz,nullzero,notnull,default:current_timestamp"`
	PasswordHash string    `bun:",nullzero,notnull"`
	Email        string    `bun:",nullzero,unique"`
	Admin        bool      `bun:",nullzero,notnull,default:false"`
}

func (u User) ToModel() models.User {
	return models.User{
		ID:           u.ID,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
		PasswordHash: u.PasswordHash,
		Email:        u.Email,
		Admin:        u.Admin,
	}
}
