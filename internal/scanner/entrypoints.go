package scanner

import (
	"go/parser"
	"go/token"
	"path/filepath"

	"github.com/clarion-dev/clarion/internal/facts"
)

// findEntrypoints scans the provided Go source files for `package main`
// declarations and returns a Component for each unique directory that
// contains at least one such file.
func findEntrypoints(goFiles []string) []facts.Component {
	seen := map[string]bool{}
	var components []facts.Component

	for _, f := range goFiles {
		pkg, line := packageName(f)
		if pkg != "main" {
			continue
		}
		dir := filepath.Dir(f)
		if seen[dir] {
			continue
		}
		seen[dir] = true

		name := filepath.Base(dir)
		components = append(components, facts.Component{
			Name:        name,
			Description: "Binary entrypoint (package main)",
			Evidence: facts.Evidence{
				SourceFiles:     []string{f},
				LineRanges:      []facts.Range{{Start: line, End: line}},
				ConfidenceScore: facts.ConfidenceDirect,
				Inferred:        false,
			},
		})
	}
	return components
}

// packageName parses only the package clause of a Go file and returns
// the package name along with the line number where it was declared.
// Returns ("", 0) on any parse error.
func packageName(filePath string) (string, int) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.PackageClauseOnly)
	if err != nil || f == nil {
		return "", 0
	}
	pos := fset.Position(f.Package)
	return f.Name.Name, pos.Line
}
