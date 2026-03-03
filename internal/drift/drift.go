// Package drift computes changes between two FactModel snapshots.
package drift

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/clarion-dev/clarion/internal/facts"
)

// warnWriter is the destination for drift warning messages; defaults to stderr.
// Overridden in tests to suppress output.
var warnWriter io.Writer = os.Stderr

// DriftReport summarises the differences between two FactModel snapshots.
type DriftReport struct {
	GeneratedAt       time.Time    `json:"generated_at"`
	PreviousSnapshot  time.Time    `json:"previous_snapshot"`
	DriftFraction     float64      `json:"drift_fraction"`
	Threshold         float64      `json:"threshold"`
	ExceededThreshold bool         `json:"exceeded_threshold"`
	Added             []DriftEntry `json:"added"`
	Removed           []DriftEntry `json:"removed"`
	Modified          []DriftEntry `json:"modified"`
	Skipped           []string     `json:"skipped,omitempty"`
}

// DriftEntry records a single changed item in a DriftReport.
type DriftEntry struct {
	Type   string `json:"type"`   // "component", "api", "datastore", "job", "integration", "config"
	Name   string `json:"name"`
	Change string `json:"change"` // "added", "removed", "modified"
}

// Compare computes a DriftReport between two FactModels.
//
// Matching key: Name + collection type.
// Modified detection: SourceFiles changed, LineRanges changed, or ConfidenceScore delta > 0.1.
// Entries with empty Name are added to Skipped with a warning.
//
// DriftFraction formula from SPEC.md §11:
//
//	total_entries = len(previous.all_entries)
//	changed = len(Added) + len(Removed) + len(Modified)
//	drift_fraction = changed / max(total_entries, 1)
func Compare(previous, current *facts.FactModel) DriftReport {
	report := DriftReport{
		GeneratedAt:      time.Now(),
		PreviousSnapshot: previous.GeneratedAt,
	}

	totalPrevious := 0

	// --- Components ---
	prevComponents := make(map[string]facts.Component, len(previous.Components))
	for _, c := range previous.Components {
		if c.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("component: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping component with empty name")
			continue
		}
		prevComponents[c.Name] = c
		totalPrevious++
	}
	currComponents := make(map[string]facts.Component, len(current.Components))
	for _, c := range current.Components {
		if c.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("component: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping component with empty name")
			continue
		}
		currComponents[c.Name] = c
	}
	for name, cc := range currComponents {
		if _, exists := prevComponents[name]; !exists {
			report.Added = append(report.Added, DriftEntry{Type: "component", Name: name, Change: "added"})
		} else {
			pc := prevComponents[name]
			if componentModified(pc.Evidence, cc.Evidence) {
				report.Modified = append(report.Modified, DriftEntry{Type: "component", Name: name, Change: "modified"})
			}
		}
	}
	for name := range prevComponents {
		if _, exists := currComponents[name]; !exists {
			report.Removed = append(report.Removed, DriftEntry{Type: "component", Name: name, Change: "removed"})
		}
	}

	// --- APIs ---
	prevAPIs := make(map[string]facts.APIEndpoint, len(previous.APIs))
	for _, a := range previous.APIs {
		if a.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("api: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping api with empty name")
			continue
		}
		prevAPIs[a.Name] = a
		totalPrevious++
	}
	currAPIs := make(map[string]facts.APIEndpoint, len(current.APIs))
	for _, a := range current.APIs {
		if a.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("api: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping api with empty name")
			continue
		}
		currAPIs[a.Name] = a
	}
	for name, ca := range currAPIs {
		if _, exists := prevAPIs[name]; !exists {
			report.Added = append(report.Added, DriftEntry{Type: "api", Name: name, Change: "added"})
		} else {
			pa := prevAPIs[name]
			if componentModified(pa.Evidence, ca.Evidence) {
				report.Modified = append(report.Modified, DriftEntry{Type: "api", Name: name, Change: "modified"})
			}
		}
	}
	for name := range prevAPIs {
		if _, exists := currAPIs[name]; !exists {
			report.Removed = append(report.Removed, DriftEntry{Type: "api", Name: name, Change: "removed"})
		}
	}

	// --- Datastores ---
	prevDatastores := make(map[string]facts.Datastore, len(previous.Datastores))
	for _, d := range previous.Datastores {
		if d.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("datastore: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping datastore with empty name")
			continue
		}
		prevDatastores[d.Name] = d
		totalPrevious++
	}
	currDatastores := make(map[string]facts.Datastore, len(current.Datastores))
	for _, d := range current.Datastores {
		if d.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("datastore: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping datastore with empty name")
			continue
		}
		currDatastores[d.Name] = d
	}
	for name, cd := range currDatastores {
		if _, exists := prevDatastores[name]; !exists {
			report.Added = append(report.Added, DriftEntry{Type: "datastore", Name: name, Change: "added"})
		} else {
			pd := prevDatastores[name]
			if componentModified(pd.Evidence, cd.Evidence) {
				report.Modified = append(report.Modified, DriftEntry{Type: "datastore", Name: name, Change: "modified"})
			}
		}
	}
	for name := range prevDatastores {
		if _, exists := currDatastores[name]; !exists {
			report.Removed = append(report.Removed, DriftEntry{Type: "datastore", Name: name, Change: "removed"})
		}
	}

	// --- Jobs ---
	prevJobs := make(map[string]facts.BackgroundJob, len(previous.Jobs))
	for _, j := range previous.Jobs {
		if j.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("job: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping job with empty name")
			continue
		}
		prevJobs[j.Name] = j
		totalPrevious++
	}
	currJobs := make(map[string]facts.BackgroundJob, len(current.Jobs))
	for _, j := range current.Jobs {
		if j.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("job: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping job with empty name")
			continue
		}
		currJobs[j.Name] = j
	}
	for name, cj := range currJobs {
		if _, exists := prevJobs[name]; !exists {
			report.Added = append(report.Added, DriftEntry{Type: "job", Name: name, Change: "added"})
		} else {
			pj := prevJobs[name]
			if componentModified(pj.Evidence, cj.Evidence) {
				report.Modified = append(report.Modified, DriftEntry{Type: "job", Name: name, Change: "modified"})
			}
		}
	}
	for name := range prevJobs {
		if _, exists := currJobs[name]; !exists {
			report.Removed = append(report.Removed, DriftEntry{Type: "job", Name: name, Change: "removed"})
		}
	}

	// --- Integrations ---
	prevIntegrations := make(map[string]facts.ExternalIntegration, len(previous.Integrations))
	for _, i := range previous.Integrations {
		if i.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("integration: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping integration with empty name")
			continue
		}
		prevIntegrations[i.Name] = i
		totalPrevious++
	}
	currIntegrations := make(map[string]facts.ExternalIntegration, len(current.Integrations))
	for _, i := range current.Integrations {
		if i.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("integration: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping integration with empty name")
			continue
		}
		currIntegrations[i.Name] = i
	}
	for name, ci := range currIntegrations {
		if _, exists := prevIntegrations[name]; !exists {
			report.Added = append(report.Added, DriftEntry{Type: "integration", Name: name, Change: "added"})
		} else {
			pi := prevIntegrations[name]
			if componentModified(pi.Evidence, ci.Evidence) {
				report.Modified = append(report.Modified, DriftEntry{Type: "integration", Name: name, Change: "modified"})
			}
		}
	}
	for name := range prevIntegrations {
		if _, exists := currIntegrations[name]; !exists {
			report.Removed = append(report.Removed, DriftEntry{Type: "integration", Name: name, Change: "removed"})
		}
	}

	// --- Config ---
	prevConfig := make(map[string]facts.ConfigVar, len(previous.Config))
	for _, cv := range previous.Config {
		if cv.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("config: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping config with empty name")
			continue
		}
		prevConfig[cv.Name] = cv
		totalPrevious++
	}
	currConfig := make(map[string]facts.ConfigVar, len(current.Config))
	for _, cv := range current.Config {
		if cv.Name == "" {
			report.Skipped = append(report.Skipped, fmt.Sprintf("config: empty name (skipped)"))
			fmt.Fprintln(warnWriter, "WARN: drift: skipping config with empty name")
			continue
		}
		currConfig[cv.Name] = cv
	}
	for name, ccv := range currConfig {
		if _, exists := prevConfig[name]; !exists {
			report.Added = append(report.Added, DriftEntry{Type: "config", Name: name, Change: "added"})
		} else {
			pcv := prevConfig[name]
			if componentModified(pcv.Evidence, ccv.Evidence) {
				report.Modified = append(report.Modified, DriftEntry{Type: "config", Name: name, Change: "modified"})
			}
		}
	}
	for name := range prevConfig {
		if _, exists := currConfig[name]; !exists {
			report.Removed = append(report.Removed, DriftEntry{Type: "config", Name: name, Change: "removed"})
		}
	}

	// Compute drift fraction.
	changed := len(report.Added) + len(report.Removed) + len(report.Modified)
	denom := totalPrevious
	if denom < 1 {
		denom = 1
	}
	report.DriftFraction = float64(changed) / float64(denom)

	return report
}

// componentModified returns true when two Evidence values are considered different.
// Criteria: SourceFiles changed, LineRanges changed, or |ConfidenceScore delta| > 0.1.
func componentModified(prev, curr facts.Evidence) bool {
	if !stringSlicesEqual(prev.SourceFiles, curr.SourceFiles) {
		return true
	}
	if !rangeSlicesEqual(prev.LineRanges, curr.LineRanges) {
		return true
	}
	delta := prev.ConfidenceScore - curr.ConfidenceScore
	if delta < 0 {
		delta = -delta
	}
	return delta > 0.1
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func rangeSlicesEqual(a, b []facts.Range) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Markdown renders a human-readable drift-report.md string.
func (r DriftReport) Markdown() string {
	var sb strings.Builder

	sb.WriteString("# Drift Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n", r.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Previous snapshot: %s\n", r.PreviousSnapshot.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Drift fraction: %.4f (threshold: %.4f)\n", r.DriftFraction, r.Threshold))

	status := "OK"
	if r.ExceededThreshold {
		status = "EXCEEDED"
	}
	sb.WriteString(fmt.Sprintf("Status: %s\n", status))

	if len(r.Added) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Added (%d)\n", len(r.Added)))
		for _, e := range r.Added {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", e.Type, e.Name))
		}
	}

	if len(r.Removed) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Removed (%d)\n", len(r.Removed)))
		for _, e := range r.Removed {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", e.Type, e.Name))
		}
	}

	if len(r.Modified) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Modified (%d)\n", len(r.Modified)))
		for _, e := range r.Modified {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", e.Type, e.Name))
		}
	}

	if len(r.Skipped) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Skipped (%d)\n", len(r.Skipped)))
		for _, s := range r.Skipped {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}

	return sb.String()
}
