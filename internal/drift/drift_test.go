package drift

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/clarion-dev/clarion/internal/facts"
)

func init() {
	// Suppress warning output during tests.
	warnWriter = io.Discard
}

// makeFactModel creates a minimal FactModel with the supplied collections for
// use in tests.
func makeFactModel(
	components []facts.Component,
	apis []facts.APIEndpoint,
	datastores []facts.Datastore,
	jobs []facts.BackgroundJob,
	integrations []facts.ExternalIntegration,
	config []facts.ConfigVar,
) *facts.FactModel {
	return &facts.FactModel{
		SchemaVersion: facts.SchemaV1,
		GeneratedAt:   time.Now(),
		Project:       facts.ProjectInfo{Name: "test"},
		Components:    components,
		APIs:          apis,
		Datastores:    datastores,
		Jobs:          jobs,
		Integrations:  integrations,
		Config:        config,
	}
}

func evidence(file string, confidence float64) facts.Evidence {
	return facts.Evidence{
		SourceFiles:     []string{file},
		LineRanges:      []facts.Range{{Start: 1, End: 10}},
		ConfidenceScore: confidence,
	}
}

// TestCompare exercises the core Compare logic with a table of cases.
func TestCompare(t *testing.T) {
	baseComponent := facts.Component{Name: "server", Evidence: evidence("main.go", 0.9)}
	baseAPI := facts.APIEndpoint{Name: "GET /users", Evidence: evidence("api.go", 0.8)}

	tests := []struct {
		name          string
		previous      *facts.FactModel
		current       *facts.FactModel
		wantAdded     int
		wantRemoved   int
		wantModified  int
		wantSkipped   int
		wantFraction  float64 // approximate
	}{
		{
			name:         "no changes",
			previous:     makeFactModel([]facts.Component{baseComponent}, nil, nil, nil, nil, nil),
			current:      makeFactModel([]facts.Component{baseComponent}, nil, nil, nil, nil, nil),
			wantAdded:    0,
			wantRemoved:  0,
			wantModified: 0,
			wantFraction: 0.0,
		},
		{
			name:     "one API added in current",
			previous: makeFactModel([]facts.Component{baseComponent}, nil, nil, nil, nil, nil),
			current: makeFactModel(
				[]facts.Component{baseComponent},
				[]facts.APIEndpoint{baseAPI},
				nil, nil, nil, nil,
			),
			wantAdded:    1,
			wantRemoved:  0,
			wantModified: 0,
			// total_previous = 1 (component only); changed = 1; fraction = 1/1 = 1.0
			wantFraction: 1.0,
		},
		{
			name:         "one component removed",
			previous:     makeFactModel([]facts.Component{baseComponent}, nil, nil, nil, nil, nil),
			current:      makeFactModel(nil, nil, nil, nil, nil, nil),
			wantAdded:    0,
			wantRemoved:  1,
			wantModified: 0,
			// total_previous = 1; changed = 1; fraction = 1/1 = 1.0
			wantFraction: 1.0,
		},
		{
			name: "one datastore modified (ConfidenceScore delta 0.2)",
			previous: makeFactModel(
				nil, nil,
				[]facts.Datastore{{Name: "postgres", Evidence: evidence("db.go", 0.9)}},
				nil, nil, nil,
			),
			current: makeFactModel(
				nil, nil,
				[]facts.Datastore{{Name: "postgres", Evidence: evidence("db.go", 0.7)}},
				nil, nil, nil,
			),
			wantAdded:    0,
			wantRemoved:  0,
			wantModified: 1,
			// total_previous = 1; changed = 1; fraction = 1.0
			wantFraction: 1.0,
		},
		{
			name: "entry with empty name skipped",
			previous: makeFactModel(
				[]facts.Component{baseComponent},
				[]facts.APIEndpoint{{Name: "", Evidence: evidence("api.go", 0.5)}},
				nil, nil, nil, nil,
			),
			current: makeFactModel(
				[]facts.Component{baseComponent},
				[]facts.APIEndpoint{{Name: "", Evidence: evidence("api.go", 0.5)}},
				nil, nil, nil, nil,
			),
			wantAdded:    0,
			wantRemoved:  0,
			wantModified: 0,
			wantSkipped:  2, // one from previous, one from current
			// total_previous = 1 (component only, api skipped); changed = 0; fraction = 0.0
			wantFraction: 0.0,
		},
		{
			// Datastore not modified when delta <= 0.1
			name: "datastore confidence delta below threshold not modified",
			previous: makeFactModel(
				nil, nil,
				[]facts.Datastore{{Name: "redis", Evidence: evidence("cache.go", 0.8)}},
				nil, nil, nil,
			),
			current: makeFactModel(
				nil, nil,
				[]facts.Datastore{{Name: "redis", Evidence: evidence("cache.go", 0.75)}},
				nil, nil, nil,
			),
			wantAdded:    0,
			wantRemoved:  0,
			wantModified: 0,
			wantFraction: 0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := Compare(tc.previous, tc.current)

			if got := len(report.Added); got != tc.wantAdded {
				t.Errorf("Added: got %d, want %d", got, tc.wantAdded)
			}
			if got := len(report.Removed); got != tc.wantRemoved {
				t.Errorf("Removed: got %d, want %d", got, tc.wantRemoved)
			}
			if got := len(report.Modified); got != tc.wantModified {
				t.Errorf("Modified: got %d, want %d", got, tc.wantModified)
			}
			if tc.wantSkipped > 0 {
				if got := len(report.Skipped); got != tc.wantSkipped {
					t.Errorf("Skipped: got %d, want %d", got, tc.wantSkipped)
				}
			}
			if got := report.DriftFraction; abs(got-tc.wantFraction) > 1e-9 {
				t.Errorf("DriftFraction: got %f, want %f", got, tc.wantFraction)
			}
		})
	}
}

// TestDriftFraction verifies the fraction computation in isolation.
func TestDriftFraction(t *testing.T) {
	tests := []struct {
		name         string
		previous     *facts.FactModel
		current      *facts.FactModel
		wantFraction float64
	}{
		{
			name: "10 total entries, 3 changed",
			previous: makeFactModel(
				makeComponents(10),
				nil, nil, nil, nil, nil,
			),
			current: makeFactModel(
				makeComponents(7), // 3 removed
				nil, nil, nil, nil, nil,
			),
			wantFraction: 0.3, // 3/10
		},
		{
			name:         "0 total entries, 0 changed — no division by zero",
			previous:     makeFactModel(nil, nil, nil, nil, nil, nil),
			current:      makeFactModel(nil, nil, nil, nil, nil, nil),
			wantFraction: 0.0, // 0/1 = 0
		},
		{
			name: "1 total, 1 added",
			previous: makeFactModel(
				[]facts.Component{{Name: "svc", Evidence: evidence("svc.go", 0.9)}},
				nil, nil, nil, nil, nil,
			),
			current: makeFactModel(
				[]facts.Component{
					{Name: "svc", Evidence: evidence("svc.go", 0.9)},
					{Name: "svc2", Evidence: evidence("svc2.go", 0.9)},
				},
				nil, nil, nil, nil, nil,
			),
			wantFraction: 1.0, // 1 added / 1 total_previous
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := Compare(tc.previous, tc.current)
			if got := report.DriftFraction; abs(got-tc.wantFraction) > 1e-9 {
				t.Errorf("DriftFraction: got %f, want %f", got, tc.wantFraction)
			}
		})
	}
}

// TestDriftMarkdown verifies that the Markdown output has the expected structure.
func TestDriftMarkdown(t *testing.T) {
	report := DriftReport{
		GeneratedAt:      time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC),
		PreviousSnapshot: time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC),
		DriftFraction:    0.25,
		Threshold:        0.20,
		ExceededThreshold: true,
		Added: []DriftEntry{
			{Type: "api", Name: "GET /users", Change: "added"},
		},
		Removed: []DriftEntry{
			{Type: "datastore", Name: "postgres-datastore", Change: "removed"},
		},
		Modified: []DriftEntry{
			{Type: "component", Name: "clarion", Change: "modified"},
		},
	}

	md := report.Markdown()

	// Must contain section headers for each non-empty section.
	for _, want := range []string{
		"# Drift Report",
		"## Added",
		"## Removed",
		"## Modified",
		"0.2500",      // drift fraction value
		"EXCEEDED",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown() missing %q\nGot:\n%s", want, md)
		}
	}

	// Entry names must appear.
	for _, want := range []string{"GET /users", "postgres-datastore", "clarion"} {
		if !strings.Contains(md, want) {
			t.Errorf("Markdown() missing entry %q\nGot:\n%s", want, md)
		}
	}
}

// TestDriftMarkdownEmptySections verifies that empty sections are omitted.
func TestDriftMarkdownEmptySections(t *testing.T) {
	report := DriftReport{
		GeneratedAt:      time.Now(),
		PreviousSnapshot: time.Now(),
		DriftFraction:    0.0,
		Threshold:        0.5,
		ExceededThreshold: false,
	}

	md := report.Markdown()

	// Empty sections should not appear.
	for _, absent := range []string{"## Added", "## Removed", "## Modified"} {
		if strings.Contains(md, absent) {
			t.Errorf("Markdown() should not contain %q for empty sections\nGot:\n%s", absent, md)
		}
	}

	// Status should be OK.
	if !strings.Contains(md, "Status: OK") {
		t.Errorf("Markdown() should contain 'Status: OK'\nGot:\n%s", md)
	}
}

// --- helpers ---

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func makeComponents(n int) []facts.Component {
	out := make([]facts.Component, n)
	for i := range out {
		out[i] = facts.Component{
			Name:     "svc" + string(rune('A'+i)),
			Evidence: evidence("svc.go", 0.9),
		}
	}
	return out
}
