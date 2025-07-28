package password

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBCryptPasswordHasher(t *testing.T) {
	hasher := NewBCryptPasswordHasher()

	t.Run("HashPassword returns non-empty string", func(t *testing.T) {
		hash, err := hasher.HashPassword("supersecret")
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
	})

	t.Run("CompareHashAndPassword succeeds on correct password", func(t *testing.T) {
		password := "supersecret"
		hash, err := hasher.HashPassword(password)
		require.NoError(t, err)

		err = hasher.CompareHashAndPassword(hash, password)
		assert.NoError(t, err)
	})

	t.Run("CompareHashAndPassword fails on incorrect password", func(t *testing.T) {
		password := "supersecret"
		wrong := "wrongpass"
		hash, err := hasher.HashPassword(password)
		require.NoError(t, err)

		err = hasher.CompareHashAndPassword(hash, wrong)
		assert.Error(t, err)
	})
}
