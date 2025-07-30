package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateAndNormalizeFediUsername verifies a username for the following criteria and returns a lowercase representation
// if valid, else an error.
// Criteria:
// - length between 3 and 64 characters
// - lowercase letters a-z, digits (1-9) and underscores (_)
// - starting with a letter
func ValidateAndNormalizeFediUsername(username string) (string, error) {
	var usernameRegex = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

	// Inside Action:
	if !usernameRegex.MatchString(username) {
		return "", fmt.Errorf("invalid username: must be 3-64 characters, start with a letter, and only contain lowercase letters, digits, or underscores")
	}

	return strings.ToLower(username), nil
}
