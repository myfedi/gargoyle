package utils

import (
	"html"

	"github.com/microcosm-cc/bluemonday"
)

var (
	ugcPolicy = bluemonday.UGCPolicy()
	strict    = bluemonday.StrictPolicy()
)

// SanitizeHTML sanitizes user-provided HTML while preserving safe formatting.
func SanitizeHTML(text string) string {
	return ugcPolicy.Sanitize(text)
}

// StripHTMLFromText splits all HTML tags from text and leaves only plain text
// Source: https://github.com/superseriousbusiness/gotosocial/blob/main/internal/text/sanitize.go
func StripHTMLFromText(text string) string {
	// Unescape first to catch any tricky critters.
	content := html.UnescapeString(text)

	// Remove all detected HTML.
	content = strict.Sanitize(content)

	// Unescape again to return plaintext.
	content = html.UnescapeString(content)
	return content
}
