package db

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewULID creates a new "Universally Unique Lexicographically Sortable Identifier" to be used
// for database ids instead of uuids. Based on entropy provided by crypto/rand and the current
// timestamp.
// See https://github.com/oklog/ulid
func NewULID() (string, error) {
	ms, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	if err != nil {
		return "", err
	}
	return ms.String(), nil
}
