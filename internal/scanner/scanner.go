// Package scanner implements the repository analysis layer for Clarion.
// It walks a Go repository, performs AST analysis, and populates a FactModel
// from structural evidence without any LLM involvement.
package scanner

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/clarion-dev/clarion/internal/facts"
)

// Scanner walks a repository root and returns a populated FactModel.
type Scanner interface {
	// Scan walks root respecting .gitignore patterns and returns a fully
	// populated FactModel. Returns an error if root is not accessible.
	Scan(root string) (*facts.FactModel, error)
}

// scanner is the concrete implementation.
type scanner struct{}

// New returns a new Scanner ready to use.
func New() Scanner {
	return &scanner{}
}

// maxWorkers caps the goroutine pool to avoid excessive parallelism on large
// machines. 8 workers saturates typical NVMe I/O without thrashing the OS
// scheduler; beyond this, gains are marginal for file-system bound workloads.
const maxWorkers = 8

// workerCount returns the goroutine pool size capped at min(NumCPU, maxWorkers).
func workerCount() int {
	n := runtime.NumCPU()
	if n > maxWorkers {
		return maxWorkers
	}
	return n
}

// Scan walks root, respects .gitignore, and returns a populated FactModel.
func (s *scanner) Scan(root string) (*facts.FactModel, error) {
	// Resolve absolute path.
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// Load the root .gitignore if present.
	gi := loadGitignore(absRoot)

	// Collect all Go files and detect languages concurrently.
	var (
		mu      sync.Mutex
		goFiles []string
		extMap  = map[string]int{}
	)

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}

		rel, relErr := filepath.Rel(absRoot, path)
		if relErr != nil {
			return nil
		}

		// Skip .git directory entirely.
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		// Check .gitignore in each directory.
		if d.IsDir() {
			dirGI := loadGitignore(path)
			gi = mergeGitignore(gi, dirGI)
		}

		if gi.Matches(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		mu.Lock()
		extMap[ext]++
		if ext == ".go" {
			goFiles = append(goFiles, path)
		}
		mu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Detect languages from extension tally.
	languages := rankLanguages(extMap)

	// Find go.mod.
	modPath := filepath.Join(absRoot, "go.mod")
	modInfo, _ := parseGoMod(modPath) // ignore error; go.mod may not exist

	// Find entrypoints (package main).
	components := findEntrypoints(goFiles)

	// Parse all .go files for AST facts using a bounded goroutine pool.
	type astResult struct {
		endpoints    []facts.APIEndpoint
		datastores   []facts.Datastore
		jobs         []facts.BackgroundJob
		integrations []facts.ExternalIntegration
		config       []facts.ConfigVar
	}

	results := make([]astResult, len(goFiles))
	sem := make(chan struct{}, workerCount())
	var wg sync.WaitGroup

	for i, f := range goFiles {
		wg.Add(1)
		go func(idx int, filePath string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			r := analyzeGoFile(filePath)
			results[idx] = astResult{
				endpoints:    r.endpoints,
				datastores:   r.datastores,
				jobs:         r.jobs,
				integrations: r.integrations,
				config:       r.config,
			}
		}(i, f)
	}
	wg.Wait()

	// Merge AST results with dependency-detected datastores/integrations.
	var (
		allEndpoints    []facts.APIEndpoint
		allDatastores   []facts.Datastore
		allJobs         []facts.BackgroundJob
		allIntegrations []facts.ExternalIntegration
		allConfig       []facts.ConfigVar
	)

	if modInfo != nil {
		allDatastores = append(allDatastores, modInfo.datastores...)
		allIntegrations = append(allIntegrations, modInfo.integrations...)
	}

	for _, r := range results {
		allEndpoints = append(allEndpoints, r.endpoints...)
		allDatastores = append(allDatastores, r.datastores...)
		allJobs = append(allJobs, r.jobs...)
		allIntegrations = append(allIntegrations, r.integrations...)
		allConfig = append(allConfig, r.config...)
	}

	// Determine project name from go.mod or directory name.
	projectName := filepath.Base(absRoot)
	goModule := ""
	if modInfo != nil && modInfo.moduleName != "" {
		goModule = modInfo.moduleName
		// Use last path segment of module path as project name.
		projectName = filepath.Base(modInfo.moduleName)
	}

	// Assemble FactModel.
	fm := buildFactModel(buildInput{
		root:         absRoot,
		projectName:  projectName,
		goModule:     goModule,
		languages:    languages,
		components:   components,
		endpoints:    allEndpoints,
		datastores:   allDatastores,
		jobs:         allJobs,
		integrations: allIntegrations,
		config:       allConfig,
	})

	return fm, nil
}
