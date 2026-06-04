package utils

import "testing"

func TestSanitizeHTMLPreservesSurroundingFormatting(t *testing.T) {
	input := " \t<p>hello    world</p>\n"
	got := SanitizeHTML(input)
	want := " \t<p>hello    world</p>\n"
	if got != want {
		t.Fatalf("SanitizeHTML() = %q, want %q", got, want)
	}
}

func TestStripHTMLFromTextPreservesSurroundingFormatting(t *testing.T) {
	input := " \t<p>hello    world</p>\n"
	got := StripHTMLFromText(input)
	want := " \thello    world\n"
	if got != want {
		t.Fatalf("StripHTMLFromText() = %q, want %q", got, want)
	}
}
