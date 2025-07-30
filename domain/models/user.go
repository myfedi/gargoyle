package models

import (
	"time"
)

type User struct {
	ID           string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	PasswordHash string
	Email        string
	Admin        bool
}
