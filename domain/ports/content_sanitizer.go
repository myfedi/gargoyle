package ports

// ContentSanitizer defines the text-normalisation operations that use cases need
// before persisting user-controlled ActivityPub note content.
//
// The interface keeps the domain layer independent from a concrete HTML
// sanitisation library while still making sanitisation an explicit business rule.
type ContentSanitizer interface {
	// SanitizeHTML returns safe HTML suitable for storing and re-serving.
	SanitizeHTML(input string) string
	// StripHTMLFromText returns a plain-text representation of HTML content.
	StripHTMLFromText(input string) string
}
