package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clarion-dev/clarion/internal/facts"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helper: parse source string and return analysis result
// ─────────────────────────────────────────────────────────────────────────────

func mustAnalyze(src string) astAnalysisResult {
	return analyzeGoSource(src)
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP Handler detection — stdlib
// ─────────────────────────────────────────────────────────────────────────────

func TestHTTPHandlerStdlib(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		wantRoute   string
		wantMethod  string
		wantHandler string
	}{
		{
			name: "http.HandleFunc",
			src: `package main
import "net/http"
func main() {
	http.HandleFunc("/hello", helloHandler)
}`,
			wantRoute:   "/hello",
			wantMethod:  "ANY",
			wantHandler: "helloHandler",
		},
		{
			name: "http.Handle",
			src: `package main
import "net/http"
func main() {
	http.Handle("/api", myHandler{})
}`,
			wantRoute:  "/api",
			wantMethod: "ANY",
		},
		{
			name: "mux.HandleFunc",
			src: `package main
import "net/http"
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", usersHandler)
}`,
			wantRoute:   "/users",
			wantMethod:  "ANY",
			wantHandler: "usersHandler",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := mustAnalyze(tc.src)
			if len(r.endpoints) == 0 {
				t.Fatalf("expected at least one endpoint, got none")
			}
			ep := r.endpoints[0]
			if ep.Route != tc.wantRoute {
				t.Errorf("route: got %q, want %q", ep.Route, tc.wantRoute)
			}
			if ep.Method != tc.wantMethod {
				t.Errorf("method: got %q, want %q", ep.Method, tc.wantMethod)
			}
			if tc.wantHandler != "" && ep.Handler != tc.wantHandler {
				t.Errorf("handler: got %q, want %q", ep.Handler, tc.wantHandler)
			}
			if ep.ConfidenceScore != facts.ConfidenceDirect {
				t.Errorf("confidence: got %v, want %v", ep.ConfidenceScore, facts.ConfidenceDirect)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP Handler detection — Gin
// ─────────────────────────────────────────────────────────────────────────────

func TestHTTPHandlerGin(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		wantRoute  string
		wantMethod string
	}{
		{
			name: "r.GET",
			src: `package main
func main() {
	r := gin.Default()
	r.GET("/users", listUsers)
}`,
			wantRoute:  "/users",
			wantMethod: "GET",
		},
		{
			name: "r.POST",
			src: `package main
func main() {
	r := gin.Default()
	r.POST("/users", createUser)
}`,
			wantRoute:  "/users",
			wantMethod: "POST",
		},
		{
			name: "r.PUT",
			src: `package main
func main() {
	r.PUT("/users/:id", updateUser)
}`,
			wantRoute:  "/users/:id",
			wantMethod: "PUT",
		},
		{
			name: "r.DELETE",
			src: `package main
func main() {
	r.DELETE("/users/:id", deleteUser)
}`,
			wantRoute:  "/users/:id",
			wantMethod: "DELETE",
		},
		{
			name: "r.PATCH",
			src: `package main
func main() {
	r.PATCH("/users/:id", patchUser)
}`,
			wantRoute:  "/users/:id",
			wantMethod: "PATCH",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := mustAnalyze(tc.src)
			if len(r.endpoints) == 0 {
				t.Fatalf("expected endpoint, got none")
			}
			ep := r.endpoints[0]
			if ep.Route != tc.wantRoute {
				t.Errorf("route: got %q, want %q", ep.Route, tc.wantRoute)
			}
			if ep.Method != tc.wantMethod {
				t.Errorf("method: got %q, want %q", ep.Method, tc.wantMethod)
			}
			if ep.ConfidenceScore != facts.ConfidenceDirect {
				t.Errorf("confidence: got %v, want %v", ep.ConfidenceScore, facts.ConfidenceDirect)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP Handler detection — Chi
// ─────────────────────────────────────────────────────────────────────────────

func TestHTTPHandlerChi(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		wantRoute  string
		wantMethod string
	}{
		{
			name: "r.Get",
			src: `package main
func main() {
	r := chi.NewRouter()
	r.Get("/items", listItems)
}`,
			wantRoute:  "/items",
			wantMethod: "GET",
		},
		{
			name: "r.Post",
			src: `package main
func main() {
	r.Post("/items", createItem)
}`,
			wantRoute:  "/items",
			wantMethod: "POST",
		},
		{
			name: "r.Delete",
			src: `package main
func main() {
	r.Delete("/items/{id}", deleteItem)
}`,
			wantRoute:  "/items/{id}",
			wantMethod: "DELETE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := mustAnalyze(tc.src)
			if len(r.endpoints) == 0 {
				t.Fatalf("expected endpoint, got none")
			}
			ep := r.endpoints[0]
			if ep.Route != tc.wantRoute {
				t.Errorf("route: got %q, want %q", ep.Route, tc.wantRoute)
			}
			if ep.Method != tc.wantMethod {
				t.Errorf("method: got %q, want %q", ep.Method, tc.wantMethod)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP Handler detection — Echo
// ─────────────────────────────────────────────────────────────────────────────

func TestHTTPHandlerEcho(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		wantRoute  string
		wantMethod string
	}{
		{
			name: "e.GET",
			src: `package main
func main() {
	e := echo.New()
	e.GET("/products", listProducts)
}`,
			wantRoute:  "/products",
			wantMethod: "GET",
		},
		{
			name: "e.POST",
			src: `package main
func main() {
	e.POST("/products", createProduct)
}`,
			wantRoute:  "/products",
			wantMethod: "POST",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := mustAnalyze(tc.src)
			if len(r.endpoints) == 0 {
				t.Fatalf("expected endpoint, got none")
			}
			ep := r.endpoints[0]
			if ep.Route != tc.wantRoute {
				t.Errorf("route: got %q, want %q", ep.Route, tc.wantRoute)
			}
			if ep.Method != tc.wantMethod {
				t.Errorf("method: got %q, want %q", ep.Method, tc.wantMethod)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// sql.Open detection
// ─────────────────────────────────────────────────────────────────────────────

func TestSQLOpenDetection(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		wantDriver     string
		wantDSNEnv     string
		wantConfidence float64
		wantInferred   bool
	}{
		{
			name: "sql.Open with env var DSN",
			src: `package main
import (
	"database/sql"
	"os"
)
func main() {
	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	_ = db
	_ = err
}`,
			wantDriver:     "postgres",
			wantDSNEnv:     "DATABASE_URL",
			wantConfidence: facts.ConfidenceDirect,
			wantInferred:   false,
		},
		{
			name: "sql.Open without traceable env",
			src: `package main
import "database/sql"
func main() {
	db, _ := sql.Open("mysql", "user:pass@/dbname")
	_ = db
}`,
			wantDriver:     "mysql",
			wantDSNEnv:     "",
			wantConfidence: facts.ConfidenceIndirect,
			wantInferred:   true,
		},
		{
			name: "sql.Open postgres",
			src: `package main
import "database/sql"
func connect() {
	sql.Open("postgres", os.Getenv("PG_DSN"))
}`,
			wantDriver: "postgres",
			wantDSNEnv: "PG_DSN",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := mustAnalyze(tc.src)
			if len(r.datastores) == 0 {
				t.Fatalf("expected datastore, got none")
			}
			ds := r.datastores[0]
			if ds.Driver != tc.wantDriver {
				t.Errorf("driver: got %q, want %q", ds.Driver, tc.wantDriver)
			}
			if tc.wantDSNEnv != "" && ds.DSNEnv != tc.wantDSNEnv {
				t.Errorf("DSNEnv: got %q, want %q", ds.DSNEnv, tc.wantDSNEnv)
			}
			if tc.wantConfidence != 0 && ds.ConfidenceScore != tc.wantConfidence {
				t.Errorf("confidence: got %v, want %v", ds.ConfidenceScore, tc.wantConfidence)
			}
			if tc.wantConfidence != 0 && ds.Inferred != tc.wantInferred {
				t.Errorf("inferred: got %v, want %v", ds.Inferred, tc.wantInferred)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// os.Getenv detection
// ─────────────────────────────────────────────────────────────────────────────

func TestEnvVarDetection(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		wantKey        string
		wantConfidence float64
	}{
		{
			name: "os.Getenv",
			src: `package main
import "os"
func main() {
	port := os.Getenv("PORT")
	_ = port
}`,
			wantKey:        "PORT",
			wantConfidence: facts.ConfidenceDirect,
		},
		{
			name: "os.LookupEnv",
			src: `package main
import "os"
func main() {
	secret, ok := os.LookupEnv("API_SECRET")
	_, _ = secret, ok
}`,
			wantKey:        "API_SECRET",
			wantConfidence: facts.ConfidenceDirect,
		},
		{
			name: "multiple env vars",
			src: `package main
import "os"
func main() {
	_ = os.Getenv("HOST")
	_ = os.Getenv("PORT")
	_ = os.Getenv("DATABASE_URL")
}`,
			wantKey:        "HOST", // first one
			wantConfidence: facts.ConfidenceDirect,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := mustAnalyze(tc.src)
			if len(r.config) == 0 {
				t.Fatalf("expected config vars, got none")
			}
			// Find the expected key.
			found := false
			for _, cv := range r.config {
				if cv.EnvKey == tc.wantKey {
					found = true
					if cv.ConfidenceScore != tc.wantConfidence {
						t.Errorf("confidence for %q: got %v, want %v",
							tc.wantKey, cv.ConfidenceScore, tc.wantConfidence)
					}
					break
				}
			}
			if !found {
				t.Errorf("env key %q not found in config vars %v", tc.wantKey, r.config)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// time.NewTicker detection
// ─────────────────────────────────────────────────────────────────────────────

func TestTickerDetection(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		wantJobs     int
		wantDirect   bool // expect ConfidenceDirect (goroutine+ticker)
	}{
		{
			name: "simple time.NewTicker",
			src: `package main
import "time"
func start() {
	ticker := time.NewTicker(5 * time.Second)
	_ = ticker
}`,
			wantJobs:   1,
			wantDirect: false,
		},
		{
			name: "goroutine with ticker loop",
			src: `package main
import "time"
func start() {
	ticker := time.NewTicker(time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				doWork()
			}
		}
	}()
}`,
			wantJobs:   1,
			wantDirect: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := mustAnalyze(tc.src)
			if len(r.jobs) < tc.wantJobs {
				t.Fatalf("expected %d jobs, got %d: %v", tc.wantJobs, len(r.jobs), r.jobs)
			}
			if tc.wantDirect {
				hasDirect := false
				for _, j := range r.jobs {
					if j.ConfidenceScore == facts.ConfidenceDirect {
						hasDirect = true
						break
					}
				}
				if !hasDirect {
					t.Errorf("expected at least one ConfidenceDirect job, got: %v", r.jobs)
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// time.AfterFunc detection
// ─────────────────────────────────────────────────────────────────────────────

func TestAfterFuncDetection(t *testing.T) {
	src := `package main
import "time"
func setup() {
	time.AfterFunc(30*time.Second, func() {
		cleanup()
	})
}`
	r := mustAnalyze(src)
	if len(r.jobs) == 0 {
		t.Fatal("expected background job from time.AfterFunc, got none")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// http.NewRequest (external integration) detection
// ─────────────────────────────────────────────────────────────────────────────

func TestHTTPNewRequestDetection(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		wantBaseURL string
	}{
		{
			name: "literal URL",
			src: `package main
import "net/http"
func call() {
	req, _ := http.NewRequest("GET", "https://api.example.com/v1/users", nil)
	_ = req
}`,
			wantBaseURL: "https://api.example.com/v1/users",
		},
		{
			name: "env var URL",
			src: `package main
import (
	"net/http"
	"os"
)
func call() {
	req, _ := http.NewRequest("POST", os.Getenv("API_BASE_URL"), nil)
	_ = req
}`,
			wantBaseURL: "$API_BASE_URL",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := mustAnalyze(tc.src)
			if len(r.integrations) == 0 {
				t.Fatalf("expected integration, got none")
			}
			found := false
			for _, ig := range r.integrations {
				if ig.BaseURL == tc.wantBaseURL {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("want BaseURL %q, got integrations: %v", tc.wantBaseURL, r.integrations)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Language detection
// ─────────────────────────────────────────────────────────────────────────────

func TestRankLanguages(t *testing.T) {
	tests := []struct {
		name      string
		extMap    map[string]int
		wantFirst string
		wantLen   int
	}{
		{
			name:      "Go dominant",
			extMap:    map[string]int{".go": 100, ".js": 10},
			wantFirst: "Go",
			wantLen:   2,
		},
		{
			name:      "mixed repo",
			extMap:    map[string]int{".py": 50, ".go": 30, ".ts": 20},
			wantFirst: "Python",
			wantLen:   3,
		},
		{
			name:      "unknown extensions ignored",
			extMap:    map[string]int{".go": 5, ".xyz": 100},
			wantFirst: "Go",
			wantLen:   1,
		},
		{
			name:    "empty map",
			extMap:  map[string]int{},
			wantLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			langs := rankLanguages(tc.extMap)
			if len(langs) != tc.wantLen {
				t.Errorf("length: got %d, want %d; langs=%v", len(langs), tc.wantLen, langs)
			}
			if tc.wantFirst != "" && (len(langs) == 0 || langs[0] != tc.wantFirst) {
				t.Errorf("first language: got %v, want %q", langs, tc.wantFirst)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// .gitignore matching
// ─────────────────────────────────────────────────────────────────────────────

func TestGitignoreMatching(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		path     string
		want     bool
	}{
		{
			name:     "exact match",
			patterns: []string{"vendor"},
			path:     "vendor",
			want:     true,
		},
		{
			name:     "glob match",
			patterns: []string{"*.log"},
			path:     "app.log",
			want:     true,
		},
		{
			name:     "glob no match",
			patterns: []string{"*.log"},
			path:     "app.go",
			want:     false,
		},
		{
			name:     "nested match",
			patterns: []string{"vendor"},
			path:     "vendor/github.com/foo/bar.go",
			want:     true,
		},
		{
			name:     "negation",
			patterns: []string{"*.log", "!important.log"},
			path:     "important.log",
			want:     false,
		},
		{
			name:     "negation does not affect other files",
			patterns: []string{"*.log", "!important.log"},
			path:     "debug.log",
			want:     true,
		},
		{
			name:     "multi-segment pattern",
			patterns: []string{"docs/internal"},
			path:     "docs/internal",
			want:     true,
		},
		{
			name:     "unmatched path",
			patterns: []string{"vendor", "*.log"},
			path:     "cmd/main.go",
			want:     false,
		},
		{
			name:     "question mark wildcard",
			patterns: []string{"?.go"},
			path:     "a.go",
			want:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gi := gitignore{}
			for _, p := range tc.patterns {
				gi.patterns = append(gi.patterns, compilePattern(p))
			}
			got := gi.Matches(tc.path)
			if got != tc.want {
				t.Errorf("Matches(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Builder deduplication
// ─────────────────────────────────────────────────────────────────────────────

func TestDeduplicateEndpoints(t *testing.T) {
	eps := []facts.APIEndpoint{
		{Name: "GET /users → list", Method: "GET", Route: "/users",
			Evidence: facts.Evidence{SourceFiles: []string{"a.go"}, ConfidenceScore: 0.7}},
		{Name: "GET /users → list2", Method: "GET", Route: "/users",
			Evidence: facts.Evidence{SourceFiles: []string{"b.go"}, ConfidenceScore: 0.9}},
	}
	result := deduplicateEndpoints(eps)
	if len(result) != 1 {
		t.Fatalf("expected 1 deduplicated endpoint, got %d", len(result))
	}
	if result[0].ConfidenceScore != 0.9 {
		t.Errorf("expected higher confidence to win, got %v", result[0].ConfidenceScore)
	}
	if len(result[0].SourceFiles) != 2 {
		t.Errorf("expected merged source files, got %v", result[0].SourceFiles)
	}
}

func TestDeduplicateConfig(t *testing.T) {
	cvs := []facts.ConfigVar{
		{Name: "PORT", EnvKey: "PORT", Evidence: facts.Evidence{SourceFiles: []string{"a.go"}, ConfidenceScore: 0.9}},
		{Name: "PORT", EnvKey: "PORT", Evidence: facts.Evidence{SourceFiles: []string{"b.go"}, ConfidenceScore: 0.9}},
		{Name: "HOST", EnvKey: "HOST", Evidence: facts.Evidence{SourceFiles: []string{"a.go"}, ConfidenceScore: 0.9}},
	}
	result := deduplicateConfig(cvs)
	if len(result) != 2 {
		t.Fatalf("expected 2 config vars, got %d: %v", len(result), result)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Entrypoint detection
// ─────────────────────────────────────────────────────────────────────────────

func TestFindEntrypoints(t *testing.T) {
	// Create a temp dir with a package main file.
	dir := t.TempDir()
	mainFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainFile, []byte(`package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// A lib file should not be detected.
	libFile := filepath.Join(dir, "lib.go")
	if err := os.WriteFile(libFile, []byte(`package mylib

func Foo() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	components := findEntrypoints([]string{mainFile, libFile})
	if len(components) != 1 {
		t.Fatalf("expected 1 component, got %d: %v", len(components), components)
	}
	if components[0].ConfidenceScore != facts.ConfidenceDirect {
		t.Errorf("confidence: got %v, want %v", components[0].ConfidenceScore, facts.ConfidenceDirect)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// go.mod parsing
// ─────────────────────────────────────────────────────────────────────────────

func TestParseGoMod(t *testing.T) {
	goModContent := `module github.com/example/myapp

go 1.22.0

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/lib/pq v1.10.9
	github.com/go-sql-driver/mysql v1.7.1
)
`
	dir := t.TempDir()
	modPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(modPath, []byte(goModContent), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := parseGoMod(modPath)
	if err != nil {
		t.Fatalf("parseGoMod: %v", err)
	}

	if info.moduleName != "github.com/example/myapp" {
		t.Errorf("moduleName: got %q, want %q", info.moduleName, "github.com/example/myapp")
	}

	// Should detect postgres and mysql drivers.
	driverNames := make(map[string]bool)
	for _, ds := range info.datastores {
		driverNames[ds.Driver] = true
	}
	if !driverNames["postgres"] {
		t.Errorf("expected postgres driver, got datastores: %v", info.datastores)
	}
	if !driverNames["mysql"] {
		t.Errorf("expected mysql driver, got datastores: %v", info.datastores)
	}

	// Should detect gin as a framework.
	found := false
	for _, fw := range info.frameworks {
		if fw == "gin" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected gin framework, got: %v", info.frameworks)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Integration test: scan the clarion repo itself
// ─────────────────────────────────────────────────────────────────────────────

func TestIntegrationScanClarionRepo(t *testing.T) {
	// Find the repo root by walking up from the test file location.
	// The test file is at internal/scanner/, so root is ../../
	root := filepath.Join("..", "..")

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's the right directory by checking for go.mod.
	if _, err := os.Stat(filepath.Join(absRoot, "go.mod")); err != nil {
		t.Skipf("cannot find clarion repo root at %s: %v", absRoot, err)
	}

	s := New()
	fm, err := s.Scan(absRoot)
	if err != nil {
		t.Fatalf("Scan(%q): %v", absRoot, err)
	}

	// Basic sanity checks.
	if fm == nil {
		t.Fatal("FactModel is nil")
	}
	if fm.SchemaVersion != facts.SchemaV1 {
		t.Errorf("SchemaVersion: got %q, want %q", fm.SchemaVersion, facts.SchemaV1)
	}
	if fm.Project.Name == "" {
		t.Error("Project.Name is empty")
	}
	if fm.Project.GoModule == "" {
		t.Error("Project.GoModule is empty")
	}
	// Clarion is a Go project — it should be detected.
	if len(fm.Project.Languages) == 0 {
		t.Error("no languages detected")
	}
	if fm.Project.Languages[0] != "Go" {
		t.Errorf("expected Go as primary language, got %v", fm.Project.Languages)
	}
	// The facts package and cmd should be detected as components.
	if len(fm.Components) == 0 {
		t.Error("no components detected (expected package main in cmd/)")
	}

	t.Logf("Scan complete: project=%q module=%q languages=%v components=%d apis=%d config=%d",
		fm.Project.Name, fm.Project.GoModule, fm.Project.Languages,
		len(fm.Components), len(fm.APIs), len(fm.Config))
}

// ─────────────────────────────────────────────────────────────────────────────
// gitignore file loading
// ─────────────────────────────────────────────────────────────────────────────

func TestLoadGitignore(t *testing.T) {
	dir := t.TempDir()
	content := `# comment
*.log
vendor/
!important.log
`
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	gi := loadGitignore(dir)
	if len(gi.patterns) != 3 { // comment lines are skipped
		t.Errorf("expected 3 patterns, got %d: %v", len(gi.patterns), gi.patterns)
	}

	// Test actual matching.
	if !gi.Matches("app.log") {
		t.Error("expected app.log to be ignored")
	}
	if !gi.Matches("vendor") {
		t.Error("expected vendor to be ignored")
	}
	if gi.Matches("important.log") {
		t.Error("expected important.log to NOT be ignored (negated)")
	}
	if gi.Matches("main.go") {
		t.Error("expected main.go to NOT be ignored")
	}
}

func TestLoadGitignoreNonexistent(t *testing.T) {
	gi := loadGitignore("/nonexistent/directory")
	if len(gi.patterns) != 0 {
		t.Errorf("expected empty patterns for nonexistent dir, got %d", len(gi.patterns))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Viper config detection
// ─────────────────────────────────────────────────────────────────────────────

func TestViperConfigDetection(t *testing.T) {
	src := `package main

func init() {
	dbHost := viper.GetString("database.host")
	dbPort := viper.GetInt("database.port")
	_ = dbHost
	_ = dbPort
}
`
	r := mustAnalyze(src)
	if len(r.config) < 2 {
		t.Fatalf("expected 2 config vars, got %d: %v", len(r.config), r.config)
	}
	keys := map[string]bool{}
	for _, cv := range r.config {
		keys[cv.EnvKey] = true
	}
	if !keys["database.host"] {
		t.Error("expected database.host config var")
	}
	if !keys["database.port"] {
		t.Error("expected database.port config var")
	}
	// Viper vars should be ConfidenceIndirect.
	for _, cv := range r.config {
		if cv.ConfidenceScore != facts.ConfidenceIndirect {
			t.Errorf("viper var %q: expected ConfidenceIndirect, got %v", cv.EnvKey, cv.ConfidenceScore)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Multiple APIs in one file
// ─────────────────────────────────────────────────────────────────────────────

func TestMultipleEndpointsInOneFile(t *testing.T) {
	src := `package main

func setupRoutes() {
	r.GET("/users", listUsers)
	r.POST("/users", createUser)
	r.GET("/users/:id", getUser)
	r.PUT("/users/:id", updateUser)
	r.DELETE("/users/:id", deleteUser)
}
`
	r := mustAnalyze(src)
	if len(r.endpoints) != 5 {
		t.Fatalf("expected 5 endpoints, got %d: %v", len(r.endpoints), r.endpoints)
	}

	methods := map[string]bool{}
	for _, ep := range r.endpoints {
		methods[ep.Method] = true
	}
	for _, expected := range []string{"GET", "POST", "PUT", "DELETE"} {
		if !methods[expected] {
			t.Errorf("expected method %q to be detected", expected)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Benchmark
// ─────────────────────────────────────────────────────────────────────────────

func BenchmarkScan(b *testing.B) {
	// Generate a synthetic repo in a temp directory.
	dir := b.TempDir()

	// Write a go.mod.
	goMod := `module github.com/bench/app

go 1.22.0

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/lib/pq v1.10.9
)
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		b.Fatal(err)
	}

	// Generate 50 packages each with 10 files of 100 lines.
	for pkg := 0; pkg < 50; pkg++ {
		pkgDir := filepath.Join(dir, fmt.Sprintf("pkg%02d", pkg))
		if err := os.MkdirAll(pkgDir, 0o755); err != nil {
			b.Fatal(err)
		}
		for file := 0; file < 10; file++ {
			src := generateSyntheticFile(pkg, file)
			name := filepath.Join(pkgDir, fmt.Sprintf("file%02d.go", file))
			if err := os.WriteFile(name, []byte(src), 0o644); err != nil {
				b.Fatal(err)
			}
		}
	}

	s := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.Scan(dir)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// generateSyntheticFile creates a Go source file with handler registrations,
// env vars, and SQL open calls to give the scanner realistic work.
func generateSyntheticFile(pkg, file int) string {
	var sb strings.Builder
	sb.WriteString("package main\n\nimport (\n\t\"net/http\"\n\t\"os\"\n\t\"database/sql\"\n)\n\n")

	for i := 0; i < 5; i++ {
		sb.WriteString("func handler")
		sb.WriteString(fmt.Sprintf("_%d_%d_%d", pkg, file, i))
		sb.WriteString("(w http.ResponseWriter, r *http.Request) {}\n")
	}

	sb.WriteString("\nfunc setup() {\n")
	for i := 0; i < 3; i++ {
		route := fmt.Sprintf("/api/pkg%d/res%d", pkg, i)
		sb.WriteString(fmt.Sprintf("\thttp.HandleFunc(%q, handler_%d_%d_%d)\n", route, pkg, file, i))
	}
	sb.WriteString(fmt.Sprintf("\t_ = os.Getenv(\"ENV_VAR_%d_%d\")\n", pkg, file))
	sb.WriteString(fmt.Sprintf("\tdb, _ := sql.Open(\"postgres\", os.Getenv(\"DB_URL_%d\"))\n", pkg))
	sb.WriteString("\t_ = db\n}\n")
	return sb.String()
}

