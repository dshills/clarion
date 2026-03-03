package scanner

import "sort"

// extToLanguage maps file extensions to canonical language names.
var extToLanguage = map[string]string{
	".go":   "Go",
	".js":   "JavaScript",
	".jsx":  "JavaScript",
	".ts":   "TypeScript",
	".tsx":  "TypeScript",
	".py":   "Python",
	".java": "Java",
	".rs":   "Rust",
	".rb":   "Ruby",
	".php":  "PHP",
	".c":    "C",
	".cpp":  "C++",
	".cc":   "C++",
	".cxx":  "C++",
	".h":    "C",
	".hpp":  "C++",
	".cs":   "C#",
	".swift": "Swift",
	".kt":   "Kotlin",
	".scala": "Scala",
	".sh":   "Shell",
	".bash": "Shell",
	".zsh":  "Shell",
}

// langCount groups extension counts by language name.
type langCount struct {
	lang  string
	count int
}

// rankLanguages takes a map of extension → file count and returns a ranked
// slice of language names, ordered by file count descending. Extensions
// not found in extToLanguage are ignored. Ties are broken alphabetically
// to make output deterministic.
func rankLanguages(extMap map[string]int) []string {
	// Aggregate by language.
	agg := map[string]int{}
	for ext, cnt := range extMap {
		if lang, ok := extToLanguage[ext]; ok {
			agg[lang] += cnt
		}
	}

	// Build sortable slice.
	ranked := make([]langCount, 0, len(agg))
	for lang, cnt := range agg {
		ranked = append(ranked, langCount{lang: lang, count: cnt})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].lang < ranked[j].lang
	})

	langs := make([]string, len(ranked))
	for i, lc := range ranked {
		langs[i] = lc.lang
	}
	return langs
}
