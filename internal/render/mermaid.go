package render

import "strings"

// ExtractMermaid finds the first ```mermaid ... ``` block in markdown.
// Returns the block content (without fences) and true, or "", false if absent.
func ExtractMermaid(markdown string) (string, bool) {
	const fence = "```mermaid"
	start := strings.Index(markdown, fence)
	if start < 0 {
		return "", false
	}
	// Find closing fence.
	rest := markdown[start+len(fence):]
	end := strings.Index(rest, "```")
	if end < 0 {
		return "", false
	}
	content := strings.TrimSpace(rest[:end])
	return content, true
}
