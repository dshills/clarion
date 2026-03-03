package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// gitignore holds compiled patterns from one or more .gitignore files.
type gitignore struct {
	patterns []pattern
}

// pattern is a single compiled .gitignore rule.
type pattern struct {
	raw      string // original line (trimmed)
	negated  bool   // starts with '!'
	dirOnly  bool   // ends with '/'
	segments []string
}

// loadGitignore reads a .gitignore file from dir and returns a gitignore.
// If no .gitignore exists in dir, an empty gitignore is returned.
func loadGitignore(dir string) gitignore {
	path := filepath.Join(dir, ".gitignore")
	f, err := os.Open(path)
	if err != nil {
		return gitignore{}
	}
	defer f.Close()

	// Pre-allocate with a reasonable capacity to avoid repeated slice growth.
	gi := gitignore{patterns: make([]pattern, 0, 32)}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		// Strip inline comments.
		if idx := strings.Index(line, " #"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimRight(line, " \t")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		gi.patterns = append(gi.patterns, compilePattern(line))
	}
	return gi
}

// mergeGitignore combines two gitignores, appending the new patterns after old.
func mergeGitignore(a, b gitignore) gitignore {
	return gitignore{patterns: append(a.patterns, b.patterns...)}
}

// compilePattern converts a raw .gitignore line into a pattern struct.
func compilePattern(raw string) pattern {
	p := pattern{raw: raw}

	s := raw
	if strings.HasPrefix(s, "!") {
		p.negated = true
		s = s[1:]
	}
	if strings.HasSuffix(s, "/") {
		p.dirOnly = true
		s = strings.TrimSuffix(s, "/")
	}
	// Remove leading slash (makes it root-relative but we handle both).
	s = strings.TrimPrefix(s, "/")

	p.segments = strings.Split(s, "/")
	return p
}

// Matches reports whether rel (a slash-cleaned relative path) is ignored.
// rel must use the OS path separator; we normalise internally.
func (gi gitignore) Matches(rel string) bool {
	// Normalise to forward slashes for matching.
	rel = filepath.ToSlash(rel)
	rel = strings.TrimPrefix(rel, "./")

	matched := false
	for _, p := range gi.patterns {
		if matchPattern(p, rel) {
			if p.negated {
				matched = false
			} else {
				matched = true
			}
		}
	}
	return matched
}

// matchPattern returns whether a single pattern matches the given slash-separated path.
func matchPattern(p pattern, rel string) bool {
	parts := strings.Split(rel, "/")

	// Single segment patterns (e.g. "*.log", "vendor") match any component.
	if len(p.segments) == 1 {
		seg := p.segments[0]
		// Match against any path component.
		for _, part := range parts {
			if matchGlob(seg, part) {
				return true
			}
		}
		return false
	}

	// Multi-segment patterns (e.g. "docs/internal") are anchored to the root.
	// Try to match the pattern segments as a prefix of the path components.
	if len(parts) < len(p.segments) {
		return false
	}
	for i, seg := range p.segments {
		if !matchGlob(seg, parts[i]) {
			// Try to find a sub-path that matches.
			return false
		}
	}
	return true
}

// matchGlob implements simple glob matching: '*' matches any sequence of
// non-separator characters, '?' matches any single character.
// It does NOT support '**'.
func matchGlob(pattern, name string) bool {
	// Fast path: no wildcards.
	if !strings.ContainsAny(pattern, "*?") {
		return pattern == name
	}

	return globMatch(pattern, name)
}

// globMatch is a recursive glob implementation for a single path segment.
func globMatch(pat, s string) bool {
	for len(pat) > 0 {
		switch pat[0] {
		case '*':
			// Consume all consecutive '*'.
			for len(pat) > 0 && pat[0] == '*' {
				pat = pat[1:]
			}
			if len(pat) == 0 {
				return true // trailing star matches rest
			}
			// Try matching the rest of the pattern at each position of s.
			for i := 0; i <= len(s); i++ {
				if globMatch(pat, s[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(s) == 0 {
				return false
			}
			pat = pat[1:]
			s = s[1:]
		default:
			if len(s) == 0 || pat[0] != s[0] {
				return false
			}
			pat = pat[1:]
			s = s[1:]
		}
	}
	return len(s) == 0
}
