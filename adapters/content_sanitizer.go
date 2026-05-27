package adapters

import "github.com/myfedi/gargoyle/utils"

// ContentSanitizer adapts the shared text utilities to the domain port.
type ContentSanitizer struct{}

func NewContentSanitizer() ContentSanitizer { return ContentSanitizer{} }

func (ContentSanitizer) SanitizeHTML(input string) string { return utils.SanitizeHTML(input) }

func (ContentSanitizer) StripHTMLFromText(input string) string { return utils.StripHTMLFromText(input) }
