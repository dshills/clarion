package generator

import (
	"strings"

	"github.com/clarion-dev/clarion/internal/facts"
)

// ApplyInferredMarkers post-processes LLM-generated Markdown by:
// - Appending " [INFERRED]" to sentences that reference entries needing the marker.
// - Deleting lines that reference entries with ConfidenceScore < 0.4 (ShouldOmit).
func ApplyInferredMarkers(text string, fm *facts.FactModel) string {
	// Collect all fact entries with their evidence.
	type entry struct {
		name     string
		score    float64
		inferred bool
	}
	var entries []entry

	for _, c := range fm.Components {
		entries = append(entries, entry{c.Name, c.ConfidenceScore, c.Inferred})
	}
	for _, a := range fm.APIs {
		entries = append(entries, entry{a.Name, a.ConfidenceScore, a.Inferred})
	}
	for _, d := range fm.Datastores {
		entries = append(entries, entry{d.Name, d.ConfidenceScore, d.Inferred})
	}
	for _, j := range fm.Jobs {
		entries = append(entries, entry{j.Name, j.ConfidenceScore, j.Inferred})
	}
	for _, i := range fm.Integrations {
		entries = append(entries, entry{i.Name, i.ConfidenceScore, i.Inferred})
	}
	for _, cv := range fm.Config {
		entries = append(entries, entry{cv.Name, cv.ConfidenceScore, cv.Inferred})
	}

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		shouldOmit := false
		needsMarker := false

		for _, e := range entries {
			if e.name == "" {
				continue
			}
			if !strings.Contains(line, e.name) {
				continue
			}
			if facts.ShouldOmit(e.score) {
				shouldOmit = true
				break
			}
			if facts.NeedsInferredMarker(e.score, e.inferred) {
				needsMarker = true
			}
		}

		if shouldOmit {
			continue
		}

		if needsMarker && !strings.HasSuffix(strings.TrimSpace(line), "[INFERRED]") {
			// Append marker before any trailing newline.
			line = strings.TrimRight(line, " ") + " [INFERRED]"
		}

		out = append(out, line)
	}

	return strings.Join(out, "\n")
}
