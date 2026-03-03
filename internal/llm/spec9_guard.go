package llm

import (
	"fmt"
	"strings"
)

// maxLeadingLines is the maximum number of lines inspected when enforcing
// SPEC.md §9. Valid prompts always lead with their content within a few lines.
const maxLeadingLines = 5

// enforceSpec9 rejects a prompt that appears to be raw Go source file content,
// complying with SPEC.md §9: LLMs must operate only on FactModel JSON,
// SPEC.md contents, and PLAN.md contents — never raw repository text.
//
// Detection: a raw Go source file's first non-blank line is always a bare
// package declaration (exactly two tokens: "package <identifier>"). Valid
// prompt sources never begin this way:
//   - FactModel JSON starts with "{"
//   - SPEC.md / PLAN.md start with a prose title
//   - Generator template output starts with "You are a …"
//
// English prose like "package main files" (three or more tokens) is not
// flagged. This structural check avoids matching literal source patterns.
func enforceSpec9(stageName Stage, prompt string) error {
	for _, line := range strings.SplitN(strings.TrimSpace(prompt), "\n", maxLeadingLines) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[0] == "package" {
			return fmt.Errorf("SPEC.md §9: stage %q prompt appears to be raw Go source "+
				"(first line is a bare package declaration %q); "+
				"prompts must contain only FactModel JSON, SPEC.md, or PLAN.md text",
				stageName, line)
		}
		break
	}
	return nil
}
