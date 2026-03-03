// Package scanner implements the repository analysis layer for Clarion.
// It walks a Go repository, performs AST-based structural analysis, and
// assembles a FactModel from evidence without any LLM involvement.
package scanner

import (
	"path/filepath"
	"sort"
	"time"

	"github.com/clarion-dev/clarion/internal/facts"
)

// buildInput groups all collected facts for assembly into a FactModel.
type buildInput struct {
	root         string
	projectName  string
	goModule     string
	languages    []string
	components   []facts.Component
	endpoints    []facts.APIEndpoint
	datastores   []facts.Datastore
	jobs         []facts.BackgroundJob
	integrations []facts.ExternalIntegration
	config       []facts.ConfigVar
}

// buildFactModel assembles, deduplicates, and sorts all collected facts
// into a canonical FactModel.
func buildFactModel(in buildInput) *facts.FactModel {
	fm := &facts.FactModel{
		SchemaVersion: facts.SchemaV1,
		GeneratedAt:   time.Now().UTC(),
		Project: facts.ProjectInfo{
			Name:      in.projectName,
			RootPath:  in.root,
			Languages: in.languages,
			GoModule:  in.goModule,
		},
		Components:   deduplicateComponents(in.components),
		APIs:         deduplicateEndpoints(in.endpoints),
		Datastores:   deduplicateDatastores(in.datastores),
		Jobs:         deduplicateJobs(in.jobs),
		Integrations: deduplicateIntegrations(in.integrations),
		Config:       deduplicateConfig(in.config),
	}

	// Make paths relative to root for portability.
	fm.Components = relativiseComponents(fm.Components, in.root)
	fm.APIs = relativiseAPIs(fm.APIs, in.root)
	fm.Datastores = relativiseDatastores(fm.Datastores, in.root)
	fm.Jobs = relativiseJobs(fm.Jobs, in.root)
	fm.Integrations = relativiseIntegrations(fm.Integrations, in.root)
	fm.Config = relativiseConfig(fm.Config, in.root)

	// Sort all slices by ConfidenceScore descending.
	sortComponents(fm.Components)
	sortEndpoints(fm.APIs)
	sortDatastores(fm.Datastores)
	sortJobs(fm.Jobs)
	sortIntegrations(fm.Integrations)
	sortConfig(fm.Config)

	return fm
}

// ── Deduplication ───────────────────────────────────────────────────────────
// Each collection is deduplicated by a string key. When duplicates exist, the
// entry with the higher ConfidenceScore wins and SourceFiles are merged.
//
// deduplicate is a generic helper that handles the map/loop boilerplate.
// key extracts the dedup key from an item; merge is called on the existing
// result entry when a duplicate is encountered.
func deduplicate[T any](items []T, key func(T) string, merge func(dst *T, src T)) []T {
	seen := map[string]int{} // key → index in result
	var result []T
	for _, item := range items {
		k := key(item)
		if idx, ok := seen[k]; ok {
			merge(&result[idx], item)
		} else {
			seen[k] = len(result)
			result = append(result, item)
		}
	}
	return result
}

// mergeEvidence merges source files from src into dst and keeps the higher
// ConfidenceScore. Used by all deduplicate calls below.
func mergeEvidence(dst *facts.Evidence, src facts.Evidence) {
	dst.SourceFiles = mergeStrings(dst.SourceFiles, src.SourceFiles)
	if src.ConfidenceScore > dst.ConfidenceScore {
		dst.ConfidenceScore = src.ConfidenceScore
		dst.Inferred = src.Inferred
	}
}

func deduplicateComponents(items []facts.Component) []facts.Component {
	return deduplicate(items,
		func(c facts.Component) string { return c.Name },
		func(dst *facts.Component, src facts.Component) {
			mergeEvidence(&dst.Evidence, src.Evidence)
		},
	)
}

func deduplicateEndpoints(items []facts.APIEndpoint) []facts.APIEndpoint {
	// Key is Method+Route, which uniquely identifies an endpoint.
	return deduplicate(items,
		func(e facts.APIEndpoint) string { return e.Method + "|" + e.Route },
		func(dst *facts.APIEndpoint, src facts.APIEndpoint) {
			mergeEvidence(&dst.Evidence, src.Evidence)
		},
	)
}

func deduplicateDatastores(items []facts.Datastore) []facts.Datastore {
	return deduplicate(items,
		func(d facts.Datastore) string { return d.Name },
		func(dst *facts.Datastore, src facts.Datastore) {
			mergeEvidence(&dst.Evidence, src.Evidence)
			// Propagate DSNEnv if the existing entry lacks it.
			if src.DSNEnv != "" && dst.DSNEnv == "" {
				dst.DSNEnv = src.DSNEnv
			}
		},
	)
}

func deduplicateJobs(items []facts.BackgroundJob) []facts.BackgroundJob {
	return deduplicate(items,
		func(j facts.BackgroundJob) string { return j.Name },
		func(dst *facts.BackgroundJob, src facts.BackgroundJob) {
			mergeEvidence(&dst.Evidence, src.Evidence)
		},
	)
}

func deduplicateIntegrations(items []facts.ExternalIntegration) []facts.ExternalIntegration {
	return deduplicate(items,
		// Use BaseURL as the dedup key when available, else Name.
		func(i facts.ExternalIntegration) string {
			if i.BaseURL != "" {
				return i.BaseURL
			}
			return i.Name
		},
		func(dst *facts.ExternalIntegration, src facts.ExternalIntegration) {
			mergeEvidence(&dst.Evidence, src.Evidence)
		},
	)
}

func deduplicateConfig(items []facts.ConfigVar) []facts.ConfigVar {
	return deduplicate(items,
		func(c facts.ConfigVar) string { return c.EnvKey },
		func(dst *facts.ConfigVar, src facts.ConfigVar) {
			mergeEvidence(&dst.Evidence, src.Evidence)
		},
	)
}

// ── Sort helpers ────────────────────────────────────────────────────────────

func sortComponents(items []facts.Component) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ConfidenceScore != items[j].ConfidenceScore {
			return items[i].ConfidenceScore > items[j].ConfidenceScore
		}
		return items[i].Name < items[j].Name
	})
}

func sortEndpoints(items []facts.APIEndpoint) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ConfidenceScore != items[j].ConfidenceScore {
			return items[i].ConfidenceScore > items[j].ConfidenceScore
		}
		return items[i].Name < items[j].Name
	})
}

func sortDatastores(items []facts.Datastore) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ConfidenceScore != items[j].ConfidenceScore {
			return items[i].ConfidenceScore > items[j].ConfidenceScore
		}
		return items[i].Name < items[j].Name
	})
}

func sortJobs(items []facts.BackgroundJob) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ConfidenceScore != items[j].ConfidenceScore {
			return items[i].ConfidenceScore > items[j].ConfidenceScore
		}
		return items[i].Name < items[j].Name
	})
}

func sortIntegrations(items []facts.ExternalIntegration) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ConfidenceScore != items[j].ConfidenceScore {
			return items[i].ConfidenceScore > items[j].ConfidenceScore
		}
		return items[i].Name < items[j].Name
	})
}

func sortConfig(items []facts.ConfigVar) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].ConfidenceScore != items[j].ConfidenceScore {
			return items[i].ConfidenceScore > items[j].ConfidenceScore
		}
		return items[i].EnvKey < items[j].EnvKey
	})
}

// ── Path relativisation ─────────────────────────────────────────────────────

func relativePath(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	return rel
}

func relativeFiles(root string, files []string) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = relativePath(root, f)
	}
	return out
}

func relativiseComponents(items []facts.Component, root string) []facts.Component {
	for i := range items {
		items[i].SourceFiles = relativeFiles(root, items[i].SourceFiles)
	}
	return items
}

func relativiseAPIs(items []facts.APIEndpoint, root string) []facts.APIEndpoint {
	for i := range items {
		items[i].SourceFiles = relativeFiles(root, items[i].SourceFiles)
	}
	return items
}

func relativiseDatastores(items []facts.Datastore, root string) []facts.Datastore {
	for i := range items {
		items[i].SourceFiles = relativeFiles(root, items[i].SourceFiles)
	}
	return items
}

func relativiseJobs(items []facts.BackgroundJob, root string) []facts.BackgroundJob {
	for i := range items {
		items[i].SourceFiles = relativeFiles(root, items[i].SourceFiles)
	}
	return items
}

func relativiseIntegrations(items []facts.ExternalIntegration, root string) []facts.ExternalIntegration {
	for i := range items {
		items[i].SourceFiles = relativeFiles(root, items[i].SourceFiles)
	}
	return items
}

func relativiseConfig(items []facts.ConfigVar, root string) []facts.ConfigVar {
	for i := range items {
		items[i].SourceFiles = relativeFiles(root, items[i].SourceFiles)
	}
	return items
}

// ── Utility ─────────────────────────────────────────────────────────────────

// mergeStrings appends elements of b to a, skipping duplicates.
func mergeStrings(a, b []string) []string {
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if !set[s] {
			a = append(a, s)
			set[s] = true
		}
	}
	return a
}
