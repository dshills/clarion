package generator

import (
	"strings"
	"unicode"
)

// noContentPhrases are phrases that indicate the LLM produced no real content.
var noContentPhrases = []string{
	"no api endpoints were found",
	"no api endpoints found",
	"no endpoints were found",
	"no endpoints found",
	"no data model",
	"no data models",
	"not available",
	"no information available",
	"no relevant information",
	"could not be determined",
	"insufficient data",
	"no source files",
	"no source code",
}

// IsSubstantive returns true if the generated text contains meaningful,
// project-specific content. It returns false for empty output, generic
// templates where every field is "UNKNOWN", or responses that explicitly
// state no content was found.
func IsSubstantive(text string) bool {
	stripped := stripMarkdownStructure(text)
	if stripped == "" {
		return false
	}

	lower := strings.ToLower(stripped)

	// Check for known "no content" phrases.
	for _, phrase := range noContentPhrases {
		if strings.Contains(lower, phrase) {
			return false
		}
	}

	// If most non-empty fields are "UNKNOWN", the output is generic.
	if isOverwhelminglyUnknown(text) {
		return false
	}

	return true
}

// stripMarkdownStructure removes headings, blank lines, horizontal rules,
// and leading/trailing whitespace, returning only the prose content.
func stripMarkdownStructure(text string) string {
	var prose []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Trim(trimmed, "-=_*") == "" {
			continue
		}
		prose = append(prose, trimmed)
	}
	return strings.Join(prose, "\n")
}

// isOverwhelminglyUnknown checks if the majority of value-like content
// in the text is just "UNKNOWN" placeholders.
func isOverwhelminglyUnknown(text string) bool {
	unknowns := 0
	values := 0

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Look for key-value patterns like "Key: UNKNOWN" or "| UNKNOWN |"
		upper := strings.ToUpper(trimmed)
		if strings.Contains(upper, "UNKNOWN") {
			unknowns++
		}
		// Count lines with actual alphanumeric content as values.
		if hasAlphanumeric(trimmed) {
			values++
		}
	}

	if values == 0 {
		return true
	}
	return values > 0 && unknowns > 0 && float64(unknowns)/float64(values) > 0.5
}

func hasAlphanumeric(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
