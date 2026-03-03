// Package verify checks generated documentation against the FactModel.
package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clarion-dev/clarion/internal/facts"
)

// Verifier checks generated documentation against a FactModel.
type Verifier struct {
	outputDir string
}

// New creates a Verifier that reads documentation from outputDir.
func New(outputDir string) *Verifier {
	return &Verifier{outputDir: outputDir}
}

// ClaimResult holds the verification result for a single sentence.
type ClaimResult struct {
	Claim       string
	MatchedName string
	Score       float64
	Supported   bool
}

// SectionReport summarizes verification results for one documentation section.
type SectionReport struct {
	Section          string
	HighConfidence   float64 // % of claims with score >= 0.7
	MediumConfidence float64 // % of claims with score in [0.4, 0.7)
	LowConfidence    float64 // % of claims with score < 0.4
	FailedClaims     []ClaimResult
}

// VerifySection checks all claims in a Markdown section against the FactModel.
// A claim is any sentence containing a Name token that appears in the FactModel.
func (v *Verifier) VerifySection(section, markdown string, fm *facts.FactModel) SectionReport {
	report := SectionReport{Section: section}

	sentences := extractSentences(markdown)
	if len(sentences) == 0 {
		return report
	}

	// Build a lookup from name → ConfidenceScore.
	type entry struct {
		name  string
		score float64
	}
	var allEntries []entry
	for _, c := range fm.Components {
		if c.Name != "" {
			allEntries = append(allEntries, entry{c.Name, c.ConfidenceScore})
		}
	}
	for _, a := range fm.APIs {
		if a.Name != "" {
			allEntries = append(allEntries, entry{a.Name, a.ConfidenceScore})
		}
	}
	for _, d := range fm.Datastores {
		if d.Name != "" {
			allEntries = append(allEntries, entry{d.Name, d.ConfidenceScore})
		}
	}
	for _, j := range fm.Jobs {
		if j.Name != "" {
			allEntries = append(allEntries, entry{j.Name, j.ConfidenceScore})
		}
	}
	for _, i := range fm.Integrations {
		if i.Name != "" {
			allEntries = append(allEntries, entry{i.Name, i.ConfidenceScore})
		}
	}
	for _, cv := range fm.Config {
		if cv.Name != "" {
			allEntries = append(allEntries, entry{cv.Name, cv.ConfidenceScore})
		}
	}

	var highCount, medCount, lowCount int

	for _, sentence := range sentences {
		// Find matching entry.
		matched := ""
		matchedScore := 0.0
		for _, e := range allEntries {
			if strings.Contains(sentence, e.name) {
				matched = e.name
				matchedScore = e.score
				break
			}
		}

		if matched == "" {
			// No FactModel match — unsupported claim.
			report.FailedClaims = append(report.FailedClaims, ClaimResult{
				Claim:     sentence,
				Supported: false,
			})
			lowCount++
			continue
		}

		if facts.IsEvidenceBacked(matchedScore) {
			highCount++
		} else if !facts.ShouldOmit(matchedScore) {
			medCount++
			report.FailedClaims = append(report.FailedClaims, ClaimResult{
				Claim:       sentence,
				MatchedName: matched,
				Score:       matchedScore,
				Supported:   false,
			})
		} else {
			lowCount++
			report.FailedClaims = append(report.FailedClaims, ClaimResult{
				Claim:       sentence,
				MatchedName: matched,
				Score:       matchedScore,
				Supported:   false,
			})
		}
	}

	total := float64(highCount + medCount + lowCount)
	if total > 0 {
		report.HighConfidence = float64(highCount) / total * 100
		report.MediumConfidence = float64(medCount) / total * 100
		report.LowConfidence = float64(lowCount) / total * 100
	}

	return report
}

// VerifyAll reads all *.md files from outputDir and verifies each against fm.
func (v *Verifier) VerifyAll(fm *facts.FactModel) ([]SectionReport, bool, error) {
	pattern := filepath.Join(v.outputDir, "*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, false, fmt.Errorf("glob %s: %w", pattern, err)
	}

	var reports []SectionReport
	allPassed := true

	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, false, fmt.Errorf("read %s: %w", path, err)
		}
		section := strings.TrimSuffix(filepath.Base(path), ".md")
		report := v.VerifySection(section, string(data), fm)
		reports = append(reports, report)
		if len(report.FailedClaims) > 0 {
			allPassed = false
		}
	}

	return reports, allPassed, nil
}

// extractSentences splits markdown into sentences, skipping code blocks and headings.
// A sentence is any non-empty line that contains at least one capitalized word.
func extractSentences(markdown string) []string {
	var sentences []string
	inCode := false

	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Only include lines with at least one word starting with uppercase.
		if containsCapitalized(trimmed) {
			sentences = append(sentences, trimmed)
		}
	}
	return sentences
}

// containsCapitalized returns true if s contains a word starting with an uppercase letter.
func containsCapitalized(s string) bool {
	for _, word := range strings.Fields(s) {
		if len(word) > 0 && word[0] >= 'A' && word[0] <= 'Z' {
			return true
		}
	}
	return false
}
