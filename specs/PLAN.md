PLAN.md

Clarion — Implementation Plan

⸻

Overview

This plan translates SPEC.md into six sequential implementation phases. Each phase has a clear entry condition, a set of deliverables, and an explicit done criteria. No phase begins until the previous phase's done criteria are met.

MVP is achieved at the end of Phase 4. Phases 5 and 6 complete the full v1 feature set.

⸻

Phase 0: Project Foundation

Goal: Establish the Go module, directory layout, CLI skeleton, and shared primitives that every subsequent phase depends on.

Entry condition: Empty repository.

0.1 Module Initialization

  • Initialize Go module: go mod init github.com/clarion-dev/clarion
  • Go 1.22+ toolchain (go.mod: go 1.22)
  • Create the directory tree from SPEC.md §13:

    /cmd/clarion/main.go
    /internal/cli/
    /internal/facts/
    /internal/scanner/
    /internal/llm/
    /internal/generator/
    /internal/render/
    /internal/verify/
    /internal/drift/

0.2 CLI Skeleton

  • Add github.com/spf13/cobra as the CLI framework. Cobra is chosen because
    it provides subcommand routing, persistent flag inheritance, and automatic
    --help generation with no external runtime dependencies beyond the stdlib,
    matching the single-static-binary constraint in SPEC.md §4.
  • Register top-level command root (clarion) with persistent global flags:
    --spec  (string, default "./SPEC.md")
    --plan  (string, default "./PLAN.md")
    --output (string, default "./docs")
    --json  (bool, default false)
    --verbose (bool, default false)
  • Enforce --json and --verbose mutual exclusion at root PersistentPreRunE.
  • In root PersistentPreRunE, after flag validation, verify that the --spec
    file exists and is readable (os.Stat + os.ReadFile check). If not:
    Fatalf("spec file not found or unreadable: %s", specPath) and exit 2.
    Exception: version command skips this check.
  • Register stub subcommands (no logic, just registered):
    pack enterprise
    gen [section]
    drift
    verify
    version
  • Wire output conventions (SPEC.md §5):
    - Progress/warnings → os.Stderr
    - Summary result → os.Stdout
    - --json mode → single JSON object to os.Stdout, no other stdout

0.3 Exit Code Constants

File: /internal/cli/exitcodes.go

  const (
      ExitSuccess  = 0
      ExitFailure  = 1   // verification/drift failure
      ExitFatal    = 2   // fatal error
  )

  All os.Exit calls in the codebase must use these constants.

0.4 Version Embedding

  • In /cmd/clarion/main.go, declare package-level vars:
      var (
          version = "dev"
          commit  = "none"
          built   = "unknown"
      )
  • Populate via -ldflags at build time:
      go build -ldflags "-X main.version=v0.1.0 -X main.commit=$(git rev-parse --short HEAD) -X main.built=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  • version command prints:
      clarion <version>
      commit: <commit>
      built:  <built>

0.5 Output Helper

File: /internal/cli/output.go

  • Logf(verbose bool, format string, args ...any) — writes to stderr if verbose.
  • Warnf(format string, args ...any) — always writes to stderr with "WARN: " prefix.
  • Fatalf(format string, args ...any) — writes to stderr with "ERROR: " prefix and calls os.Exit(ExitFatal).
  • Summary(files []string) — prints one line per file to stdout (suppressed in --json mode).

0.6 Output Directory Lock

File: /internal/cli/lock.go

  To prevent concurrent execution against the same --output directory (which
  SPEC.md §5 defines as producing undefined behavior), implement a file-based
  advisory lock:

  func AcquireLock(outputDir string) (unlock func(), err error)
    - Creates outputDir/.clarion.lock using os.OpenFile with O_CREATE|O_EXCL.
    - If the file already exists: Fatalf("Another clarion process is running
      against %s. If no process is running, delete .clarion.lock and retry.")
    - Returns an unlock func that removes the lock file; caller must defer it.
    - All commands that write to --output (pack, gen, drift) must call
      AcquireLock before doing any work.
    - verify and version do not write output and do not acquire the lock.

0.7 Makefile

  Targets:
    build       go build -ldflags "..." -o bin/clarion ./cmd/clarion
    test        go test ./...
    lint        golangci-lint run ./...
    clean       rm -rf bin/

Done criteria for Phase 0:
  ✓ go build ./cmd/clarion produces a binary.
  ✓ clarion version prints version/commit/built.
  ✓ clarion --json --verbose fails with a usage error.
  ✓ All stub subcommands run and exit 0 without panicking.
  ✓ AcquireLock returns an error when the lock file already exists.
  ✓ go test ./... passes (no tests yet beyond lock, just compilation check).

⸻

Phase 1: Fact Model Types and Serialization

Goal: Define the canonical internal data model (SPEC.md §8) as Go types with full JSON serialization, deserialization, and the evidence-backed rules (SPEC.md §8.1).

Entry condition: Phase 0 done.

1.1 Core Types

File: /internal/facts/types.go

  type Range struct {
      Start int `json:"start"`
      End   int `json:"end"`
  }

  type Evidence struct {
      SourceFiles     []string `json:"source_files"`
      LineRanges      []Range  `json:"line_ranges"`
      ConfidenceScore float64  `json:"confidence_score"`
      Inferred        bool     `json:"inferred"`
  }

  type ProjectInfo struct {
      Name        string   `json:"name"`
      RootPath    string   `json:"root_path"`
      Languages   []string `json:"languages"`
      GoModule    string   `json:"go_module,omitempty"`
      Description string   `json:"description,omitempty"`
  }

  type Component struct {
      Name        string   `json:"name"`
      Description string   `json:"description,omitempty"`
      Evidence
  }

  type APIEndpoint struct {
      Name        string `json:"name"`
      Method      string `json:"method"`
      Route       string `json:"route"`
      AuthPattern string `json:"auth_pattern,omitempty"`
      Handler     string `json:"handler,omitempty"`
      Evidence
  }

  type Datastore struct {
      Name   string `json:"name"`
      Driver string `json:"driver,omitempty"`
      DSNEnv string `json:"dsn_env,omitempty"`
      Evidence
  }

  type BackgroundJob struct {
      Name     string `json:"name"`
      Schedule string `json:"schedule,omitempty"`
      Evidence
  }

  type ExternalIntegration struct {
      Name     string `json:"name"`
      BaseURL  string `json:"base_url,omitempty"`
      Evidence
  }

  type ConfigVar struct {
      Name         string `json:"name"`
      EnvKey       string `json:"env_key"`
      DefaultValue string `json:"default_value,omitempty"`
      Required     bool   `json:"required"`
      Evidence
  }

  type SecurityModel struct {
      AuthMechanism string   `json:"auth_mechanism,omitempty"`
      TrustBoundaries []string `json:"trust_boundaries,omitempty"`
      Evidence
  }

  type FactModel struct {
      SchemaVersion string                `json:"schema_version"`
      GeneratedAt   time.Time             `json:"generated_at"`
      Project       ProjectInfo           `json:"project"`
      Components    []Component           `json:"components"`
      APIs          []APIEndpoint         `json:"apis"`
      Datastores    []Datastore           `json:"datastores"`
      Jobs          []BackgroundJob       `json:"jobs"`
      Integrations  []ExternalIntegration `json:"integrations"`
      Config        []ConfigVar           `json:"config"`
      Security      SecurityModel         `json:"security"`
  }

  SchemaVersion must be set to "1.0" for all v1 output.

  Expected serialized size: well under 200KB for typical projects (tens of
  components, hundreds of API endpoints). The 200KB cap (SPEC.md §12.2) is
  enforced by TruncateToSize (§1.4) before any LLM call.

  TruncateToSize drop order: APIs → Datastores → Jobs → Integrations → Config
  (ascending ConfidenceScore within each collection). Components and Security
  are never dropped.

  Edge case — still over 200KB after all droppable entries removed: this means
  the Project, Components, and Security fields alone exceed 200KB (extremely
  unlikely in practice). Behavior:
  1. Log to stderr: "WARN: FactModel still %d bytes after dropping all droppable entries."
  2. Log the top 5 largest Component names and their estimated byte contribution.
  3. Fatalf("FactModel too large to send to LLM even after truncation (%d bytes).
     Consider scanning a subdirectory instead of the whole repository, or contact
     support.") and exit 2. No output files are written.

1.2 Confidence Score Constants

File: /internal/facts/confidence.go

  const (
      ConfidenceDirect    = 0.9  // explicitly registered/declared
      ConfidenceIndirect  = 0.7  // matches known pattern, no explicit registration
      ConfidenceInferred  = 0.5  // naming convention or partial match
      ConfidenceSpeculative = 0.2 // keyword/comment only, no AST proof
  )

  // IsEvidenceBacked returns true if the score meets the evidence-backed threshold.
  func IsEvidenceBacked(score float64) bool { return score >= 0.7 }

  // ShouldOmit returns true if the score is too low to include in output.
  func ShouldOmit(score float64) bool { return score < 0.4 }

  // NeedsInferredMarker returns true if the claim must carry [INFERRED] in output.
  func NeedsInferredMarker(score float64, inferred bool) bool {
      return inferred || (score >= 0.4 && score < 0.7)
  }

1.3 Serialization

File: /internal/facts/store.go

  // Load reads and deserializes clarion-meta.json from the given path.
  // Returns error if the file is missing, unreadable, or invalid JSON.
  func Load(path string) (*FactModel, error)

  // Save serializes the FactModel to the given path as pretty-printed JSON.
  // Creates parent directories if absent.
  func Save(path string, fm *FactModel) error

  // Validate checks that all required fields are present and SchemaVersion matches.
  func Validate(fm *FactModel) error

1.4 Size Enforcement

File: /internal/facts/truncate.go

  // TruncateToSize returns a copy of fm with the lowest-confidence entries
  // removed until the JSON serialization fits within maxBytes (default 200*1024).
  // Logs a warning listing the count of dropped entries.
  func TruncateToSize(fm *FactModel, maxBytes int) (*FactModel, int, error)

  The function drops from: APIs, Datastores, Jobs, Integrations, Config entries
  in ascending ConfidenceScore order until the target size is met.
  Components and Security are never dropped.

1.5 Tests

File: /internal/facts/facts_test.go

  Table-driven tests covering:
  - Load/Save round-trip (temp file).
  - Validate catches missing Name or SourceFiles.
  - TruncateToSize drops correct entries and returns drop count.
  - IsEvidenceBacked, ShouldOmit, NeedsInferredMarker boundary values.
  - JSON output includes schema_version and generated_at.

Done criteria for Phase 1:
  ✓ go test ./internal/facts/... passes.
  ✓ A FactModel round-trips through JSON without data loss.
  ✓ TruncateToSize reduces a >200KB model below the limit.
  ✓ Validate rejects models with empty Name or nil SourceFiles.

⸻

Phase 2: Repository Scanner

Goal: Implement the scanner layer (SPEC.md §7) that walks a Go repository and populates a FactModel from AST evidence. No LLM involvement.

Entry condition: Phase 1 done.

2.1 Scanner Interface

File: /internal/scanner/scanner.go

  type Scanner interface {
      // Scan walks root and returns a populated FactModel.
      // Respects .gitignore patterns found anywhere in the tree.
      Scan(root string) (*facts.FactModel, error)
  }

  func New() Scanner

2.2 .gitignore Support

File: /internal/scanner/gitignore.go

  • On each directory entry, load .gitignore if present.
  • Apply patterns using github.com/go-git/go-git/v5/plumbing/format/gitignore or a pure-Go equivalent.
  • Skip ignored paths entirely (do not parse them).

2.3 Language Detection

File: /internal/scanner/language.go

  • Walk the file tree and tally extensions.
  • Return a ranked []string of language names.
  • Current supported detection: Go (*.go), plus passive detection of JS/TS/Python/Java/Rust by extension count.
  • Only Go receives deep AST analysis in v1.

2.4 Entrypoint Detection

File: /internal/scanner/entrypoints.go

  • Identify package main files.
  • For each, record the directory path as a Component with:
      ConfidenceScore = ConfidenceDirect
      Inferred = false
  • Source: file path + line 1 (package main declaration).

2.5 Dependency Manifest Parser

File: /internal/scanner/deps.go

  • Parse go.mod using golang.org/x/mod/modfile.
  • Extract module name → ProjectInfo.GoModule.
  • Extract direct dependencies → ExternalIntegration entries with ConfidenceScore = ConfidenceDirect.
  • Detect known database drivers, HTTP frameworks, and cloud SDKs by import path prefix:

      Database drivers:
        github.com/lib/pq          → Datastore{Driver: "postgres"}
        github.com/go-sql-driver/mysql → Datastore{Driver: "mysql"}
        github.com/mattn/go-sqlite3 → Datastore{Driver: "sqlite3"}
        go.mongodb.org/mongo-driver → Datastore{Driver: "mongodb"}

      HTTP frameworks (used to tune handler detection heuristics):
        github.com/gin-gonic/gin   → framework: gin
        github.com/go-chi/chi      → framework: chi
        github.com/labstack/echo   → framework: echo
        net/http (stdlib)          → framework: stdlib

2.6 Go AST Parser

File: /internal/scanner/goast.go

  Use go/parser and go/ast. Parse each .go file with parser.ParseFile
  using mode parser.ParseComments.

  2.6.1 HTTP Handler Detection

    For stdlib / chi / gin / echo, identify handler registrations:

    Stdlib pattern:
      http.HandleFunc("/path", handlerFn)
      http.Handle("/path", handler)
      mux.HandleFunc("/path", handlerFn)
    → APIEndpoint{Method: "ANY", Route: "/path", Handler: "handlerFn",
                  ConfidenceScore: ConfidenceDirect}

    Gin pattern:
      r.GET("/path", handler)  r.POST("/path", handler)  etc.
    → APIEndpoint{Method: "GET", Route: "/path", ...}

    Chi pattern:
      r.Get("/path", handler)  r.Post("/path", handler)  etc.

    Echo pattern:
      e.GET("/path", handler)  e.POST("/path", handler)  etc.

    If a handler function is defined but not found in any registration:
    → APIEndpoint{ConfidenceScore: ConfidenceInferred, Inferred: true}

  2.6.2 Struct Tag Analysis

    For each struct field with json or db tags:
    - Record the struct as a Datastore or data transfer type.
    - Collect field name, json key, db column name.
    - Apply PII heuristics: if field name matches /email|phone|ssn|dob|address|password/i,
      mark as potential PII in Description.

  2.6.3 Database Usage Detection

    - Detect sql.Open, gorm.Open, mongo.Connect, etc. call expressions.
    - Extract DSN env variable name from os.Getenv("KEY") arguments.
    → Datastore{DSNEnv: "KEY", ConfidenceScore: ConfidenceDirect}

    - If sql.Open is detected without a traceable env var:
    → Datastore{ConfidenceScore: ConfidenceIndirect, Inferred: true}

  2.6.4 Environment Variable / Config Detection

    - Detect os.Getenv("KEY") and os.LookupEnv("KEY") call expressions.
    → ConfigVar{EnvKey: "KEY", ConfidenceScore: ConfidenceDirect}

    - Detect viper.GetString("key") or similar:
    → ConfigVar{EnvKey: "key", ConfidenceScore: ConfidenceIndirect}

  2.6.5 Background Job Detection

    - Detect time.NewTicker, time.AfterFunc, cron.New() usages.
    → BackgroundJob{ConfidenceScore: ConfidenceIndirect, Inferred: true}

    - Detect goroutine function literals with for { select { case <-ticker.C } }:
    → BackgroundJob{ConfidenceScore: ConfidenceDirect}

  2.6.6 External Integration Detection

    - Detect http.NewRequest or http.Get with a URL literal or env-var base URL.
    → ExternalIntegration{BaseURL: "<literal or env var>", ConfidenceScore: ConfidenceIndirect}

2.7 FactModel Assembly

File: /internal/scanner/builder.go

  After walking all files:
  - Deduplicate entries by Name within each collection.
  - Sort all slices by ConfidenceScore descending (highest first).
  - Set Project.Languages from language detection results.
  - Set Project.Name from go.mod module name (last path segment).
  - Assign SourceFiles and LineRanges to each entry from AST position data.

2.8 Performance

  - Use sync.WaitGroup to walk packages concurrently (one goroutine per top-level package).
  - Limit goroutine pool to min(runtime.NumCPU(), 8).
  - Scanning 200k LOC must complete under 10 seconds on a reference machine
    defined as: 4-core x86-64 CPU (e.g. Apple M1 or equivalent Intel/AMD),
    16 GB RAM, NVMe SSD. This matches a typical developer laptop as of 2024.
    BenchmarkScanLargeRepo (Phase 6.4) enforces this bound with t.Fatal.

2.9 Tests

  - Unit tests for each detector (goast_test.go) using synthetic Go source strings parsed in-memory.
  - Table-driven tests: input Go snippet → expected []APIEndpoint / []Datastore / etc.
  - Integration test: scan the clarion repo itself and assert non-empty FactModel.
  - Benchmark test (BenchmarkScan) with a synthetic large repo fixture.

Done criteria for Phase 2:
  ✓ go test ./internal/scanner/... passes.
  ✓ Scanning a Go repo with gin routes produces APIEndpoint entries with Method populated.
  ✓ Scanning a repo with sql.Open detects a Datastore.
  ✓ .gitignore patterns exclude matched paths from scanning.
  ✓ BenchmarkScan does not exceed 10s for a 200k LOC fixture.

⸻

Phase 3: LLM Integration Layer

Goal: Implement the LLM client (SPEC.md §12) with OpenAI and Anthropic adapters, token budget tracking, retry logic, and a mock adapter for deterministic tests.

Entry condition: Phase 1 done (Phase 2 not required; phases 2 and 3 can be developed in parallel).

3.1 Types

File: /internal/llm/types.go

  Implement LLMRequest, LLMResponse, and ProviderAdapter exactly as specified in SPEC.md §12.2.

  Additional type:

  type Stage string
  const (
      StageSummarize  Stage = "summarize"
      StageGenerate   Stage = "generate"
      StageVerify     Stage = "verify"
  )

  type StageResult struct {
      Stage            Stage
      Response         LLMResponse
      CumulativeTokens int
  }

3.2 Config Loader

File: /internal/llm/config.go

  type Config struct {
      Provider    string  `json:"provider"`            // CLARION_LLM_PROVIDER; required; one of "openai","anthropic"
      Model       string  `json:"model"`               // CLARION_LLM_MODEL; required; non-empty string
      APIKey      string  `json:"-"`                   // CLARION_LLM_API_KEY; required; never serialized
      TokenBudget int     `json:"token_budget"`        // CLARION_LLM_TOKEN_BUDGET; default 100000; must be > 0
  }

  func LoadConfig() (Config, error)
    Validation rules (all checked before returning; errors are accumulated and
    returned as a single multi-line error if more than one fails):
    - CLARION_LLM_PROVIDER unset or empty → "CLARION_LLM_PROVIDER is required (openai or anthropic)"
    - CLARION_LLM_PROVIDER not in {"openai","anthropic"} → "CLARION_LLM_PROVIDER must be one of: openai, anthropic"
    - CLARION_LLM_MODEL unset or empty → "CLARION_LLM_MODEL is required"
    - CLARION_LLM_API_KEY unset or empty → "CLARION_LLM_API_KEY is required"
    - CLARION_LLM_TOKEN_BUDGET set but <= 0 → "CLARION_LLM_TOKEN_BUDGET must be > 0"
    - CLARION_LLM_TOKEN_BUDGET set but not a valid integer → "CLARION_LLM_TOKEN_BUDGET must be an integer"

3.3 Token Budget Tracker

File: /internal/llm/budget.go

  type BudgetTracker struct {
      limit int
      used  int
  }

  func NewBudgetTracker(limit int) *BudgetTracker
  func (b *BudgetTracker) Used() int
  func (b *BudgetTracker) Remaining() int
  func (b *BudgetTracker) CanAfford(estimated int) bool
  func (b *BudgetTracker) Record(tokens int)

  EstimateTokens(text string) int
    - Approximation: len(text)/4 (standard GPT token approximation).
    - Used only for pre-stage budget checks; actual usage is recorded from LLMResponse.

3.4 OpenAI Adapter

File: /internal/llm/openai.go

  Implements ProviderAdapter.
  HTTP client: net/http stdlib only (no SDK dependency).
  Endpoint: POST https://api.openai.com/v1/chat/completions
  Request body: {"model": ..., "temperature": 0, "max_tokens": ...,
                  "messages": [{"role": "user", "content": <Prompt>}]}
  Parse response: choices[0].message.content → Text,
                  usage.prompt_tokens → PromptTokens,
                  usage.completion_tokens → CompletionTokens,
                  model → ModelID.
  Measure latency from request send to first byte received.

3.5 Anthropic Adapter

File: /internal/llm/anthropic.go

  Implements ProviderAdapter.
  Endpoint: POST https://api.anthropic.com/v1/messages
  Headers: x-api-key, anthropic-version: 2023-06-01
  Request body: {"model": ..., "temperature": 0, "max_tokens": ...,
                  "messages": [{"role": "user", "content": <Prompt>}]}
  Parse response: content[0].text → Text,
                  usage.input_tokens → PromptTokens,
                  usage.output_tokens → CompletionTokens,
                  model → ModelID.

3.6 Error Handling

File: /internal/llm/errors.go

  Implement all error handling rules from SPEC.md §12.3:
  - HTTP 429, 503, 504, timeout → retry once after 2s.
  - HTTP 401, 403 → return error immediately.
  - Second consecutive failure → return wrapped error including provider + model + status.
  - Non-JSON or truncated response body → return error with message:
      "provider <name>: unparseable response (model=<model>): <first 200 chars of body>"
  - context.DeadlineExceeded, net.Error → return error with full message.

3.7 Pipeline Runner

File: /internal/llm/pipeline.go

  type Pipeline struct {
      adapter ProviderAdapter
      budget  *BudgetTracker
      verbose bool
  }

  func (p *Pipeline) Run(ctx context.Context, stages []PipelineStage) ([]StageResult, error)

  type PipelineStage struct {
      Name     Stage
      Prompt   string
      Required bool  // if true, budget exhaustion before this stage is fatal (exit 2)
  }

  Stage assignment:
  - StageSummarize: Required = false  (optional; skipping degrades quality but is non-fatal)
  - StageGenerate:  Required = true   (skipping means no output; must treat as fatal)
  - StageVerify:    Required = false  (skipping means unverified output; non-fatal, emit warning)

  Run logic:
  1. Before stage 1: validate budget > 0 per SPEC.md §12.1.
  2. For each stage:
     a. EstimateTokens(prompt).
     b. If !CanAfford(estimate):
          - If Required: Warnf("Token budget too small for required stage %s. Increase CLARION_LLM_TOKEN_BUDGET."); return nil, ErrBudgetExhausted (caller exits 2).
          - If !Required: Warnf("Token budget exceeded: %d/%d tokens. Skipping stage %s and all remaining stages.", used, limit, stage.Name); write all completed output files to disk; return completed StageResults, ErrBudgetSkipped (caller exits 1).
     c. Call adapter.Call(ctx, req).
     d. Record actual tokens via budget.Record(resp.PromptTokens + resp.CompletionTokens).
     e. Log per-stage: stage name, tokens used, cumulative tokens, estimated cost, duration ms.
  3. Return all StageResult values (including skipped stages with zero-value LLMResponse).

  Cost estimation: use a conservative $0.01/1K tokens for logging (not billing).

3.8 Mock Adapter

File: /internal/llm/mock.go

  type MockAdapter struct {
      Responses map[string]LLMResponse  // keyed by prompt substring or stage name
      CallCount int
  }
  Implements ProviderAdapter. Returns pre-configured responses deterministically.
  Used in all generator and verify tests.

3.9 Adapter Factory

File: /internal/llm/factory.go

  func NewAdapter(cfg Config) (ProviderAdapter, error)
    - Returns OpenAI or Anthropic adapter based on cfg.Provider.
    - Calls Validate() on the adapter; returns error if validation fails.

3.10 Tests

  - TestBudgetTracker: table-driven, covering CanAfford, Record, boundary at 0.
  - TestOpenAIAdapter: mock HTTP server returning canned responses; verify Text,
    PromptTokens, CompletionTokens, ModelID parsed correctly.
  - TestAnthropicAdapter: same as above for Anthropic response shape.
  - TestErrorHandling: table-driven with one case per SPEC.md §12.3 rule:
      (a) HTTP 429 → retry once; second call succeeds → no error returned.
      (b) HTTP 429 → retry once; second call also 429 → error with status 429.
      (c) HTTP 401 → error returned immediately, no retry.
      (d) HTTP 503 → retry once; second call succeeds → no error.
      (e) Malformed JSON body → error message contains "unparseable response".
      (f) context.DeadlineExceeded → error message contains "DeadlineExceeded".
  - TestPipeline: mock adapter, covering:
      all stages complete → StageResults has 3 entries, error nil.
      mid-run budget exhaustion on optional stage → ErrBudgetSkipped, exit 1.
      pre-Stage-1 budget zero → error returned before any adapter call, exit 2.
      required stage budget insufficient → ErrBudgetExhausted, exit 2.

Done criteria for Phase 3:
  ✓ go test ./internal/llm/... passes with zero failures.
  ✓ LoadConfig returns a descriptive error when any required env var is missing.
  ✓ BudgetTracker correctly blocks stages that exceed the remaining budget,
    including the pre-Stage-1 zero-budget and insufficient-budget checks.
  ✓ OpenAI adapter correctly parses choices[0].message.content and usage fields.
  ✓ Anthropic adapter correctly parses content[0].text and usage fields.
  ✓ Retry fires exactly once on 503; does not retry on 401.
  ✓ MockAdapter returns pre-configured responses deterministically across
    10 consecutive calls with identical inputs (regression test for determinism).
  ✓ TestPipeline covers: all stages complete, mid-run budget exhaustion (exit 1),
    pre-Stage-1 budget too small (exit 2).

⸻

Phase 4: Documentation Generator, Output Renderer, and Verification Engine (MVP)

Goal: Generate architecture.md and clarion-meta.json, implement the verify command, and reach MVP. This phase integrates phases 1–3 into the first working end-to-end pipeline.

Entry condition: Phase 1 done criteria met AND Phase 2 done criteria met AND
Phase 3 done criteria met. All three phases must be fully green before Phase 4
work begins; Phase 4 integrates all three layers and cannot function if any
layer is incomplete.

Phase gate: before starting Phase 4, run `go test ./internal/facts/...
./internal/scanner/... ./internal/llm/...` and confirm all pass. Record the
green test run as the gate artifact (e.g., CI run URL or local log).

4.1 Prompt Templates

Directory: /internal/generator/templates/

  One file per document section:
    architecture.tmpl
    api.tmpl
    data-model.tmpl
    runbook.tmpl

  Each template receives:
    .FactModel   — JSON string of the (possibly truncated) FactModel
    .Spec        — contents of SPEC.md
    .Plan        — contents of PLAN.md (empty string if absent)

  Template format: Go text/template.

  architecture.tmpl must instruct the LLM to produce:
    - System overview
    - Component breakdown with source evidence
    - Data flow explanation
    - External dependencies
    - Trust boundaries
    - A Mermaid component diagram (```mermaid graph TD ... ```)
    - All claims prefixed with the source file path

  api.tmpl must instruct the LLM to:
    - List only endpoints present in .FactModel.APIs
    - Include Method, Route, Handler, AuthPattern
    - Not fabricate endpoints absent from the FactModel

  data-model.tmpl must instruct the LLM to:
    - Describe each Datastore and its fields
    - Note PII-flagged fields explicitly
    - Produce a Mermaid ER diagram (```mermaid erDiagram ... ```)

  runbook.tmpl must instruct the LLM to:
    - List startup instructions based on entrypoints
    - List all ConfigVar entries as required environment variables
    - List external dependencies
    - Write UNKNOWN for any field that cannot be derived from the FactModel

4.2 Generator

File: /internal/generator/generator.go

  type Generator struct {
      pipeline *llm.Pipeline
      templates *template.Template
  }

  func New(pipeline *llm.Pipeline) *Generator

  func (g *Generator) GenerateSection(ctx context.Context, section string, fm *facts.FactModel,
      spec, plan string) (string, error)
    - section: one of "architecture", "api", "data-model", "runbook"
    - Renders the appropriate template.
    - Calls pipeline.Run with stages: [StageGenerate].
    - Returns the LLM-produced Markdown text.

  func (g *Generator) GenerateAll(ctx context.Context, fm *facts.FactModel,
      spec, plan string) (map[string]string, error)
    - Calls GenerateSection for each of the four sections.
    - Applies [INFERRED] markers per §8.1 rules (see §4.3 below).
    - Returns map[section]markdown.

4.3 [INFERRED] Marker Injection

File: /internal/generator/markers.go

  After receiving LLM output for a section, post-process the Markdown:
  - For every claim sentence that references a FactModel entry with
    NeedsInferredMarker == true, append " [INFERRED]" to that sentence.
  - Implementation: scan generated text for known entry Names; if the entry
    has Inferred=true or ConfidenceScore in [0.4, 0.7), append the marker
    to the sentence containing the Name.
  - Claims from entries with ConfidenceScore < 0.4 must be stripped from
    the output (regex-delete lines containing the entry Name).

4.4 Output Renderer

File: /internal/render/renderer.go

  type Renderer struct {
      outputDir string
      jsonMode  bool
  }

  func New(outputDir string, jsonMode bool) *Renderer

  func (r *Renderer) WriteMarkdown(filename, content string) error
    - Writes to outputDir/filename.
    - Creates outputDir if absent.
    - Prints "Wrote: <path>" to stdout (unless --json).

  func (r *Renderer) WriteMermaid(filename, diagram string) error
    - Writes to outputDir/diagrams/filename.
    - Extracts the ```mermaid ... ``` block from generated Markdown if present.

  func (r *Renderer) WriteFactModel(fm *facts.FactModel) error
    - Calls facts.Save to outputDir/clarion-meta.json.

  func (r *Renderer) WriteJSON(result any) error
    - Marshals result to JSON and prints to stdout (--json mode).

4.5 Mermaid Extraction

File: /internal/render/mermaid.go

  func ExtractMermaid(markdown string) (string, bool)
    - Finds the first ```mermaid ... ``` block.
    - Returns the block content (without fences) and true, or "", false if absent.

  Extracted Mermaid blocks are saved as separate .mmd files in addition to
  remaining embedded in the Markdown.

4.6 Verification Engine

File: /internal/verify/verify.go

  type Verifier struct {
      outputDir string
  }

  func New(outputDir string) *Verifier

  type ClaimResult struct {
      Claim       string
      MatchedName string
      Score       float64
      Supported   bool
  }

  type SectionReport struct {
      Section          string
      HighConfidence   float64  // % of claims with score >= 0.7
      MediumConfidence float64  // % of claims with score in [0.4, 0.7)
      LowConfidence    float64  // % of claims with score < 0.4
      FailedClaims     []ClaimResult
  }

  func (v *Verifier) VerifySection(section, markdown string, fm *facts.FactModel) SectionReport
    Algorithm:
    1. Extract all sentences from markdown that contain a capitalized Name token.
    2. For each sentence, search fm for an entry whose Name appears in the sentence.
    3. If no match: ClaimResult{Supported: false}.
    4. If match found but score < 0.7: ClaimResult{Supported: false, Score: match.Score}.
    5. Compute confidence percentages across all claims.

  func (v *Verifier) VerifyAll(fm *facts.FactModel) ([]SectionReport, bool, error)
    - Reads each *.md file from outputDir.
    - Calls VerifySection for each.
    - Returns reports, allPassed bool, error.
    - allPassed = true only when all sections have zero FailedClaims.

4.7 Verify Command

File: /internal/cli/verify.go

  Pre-checks (SPEC.md §5 out-of-order):
  - If outputDir has no *.md files: Fatalf("No documentation found in %s. Run clarion pack enterprise first.")
  - If clarion-meta.json absent: Fatalf("clarion-meta.json not found in %s. Run clarion pack enterprise first.")

  Run VerifyAll. Print per-section confidence percentages to stdout.
  If !allPassed: os.Exit(ExitFailure).

4.8 pack enterprise Command (MVP Slice)

File: /internal/cli/pack.go

  Sequence:
  1. Validate --spec exists and is readable.
  2. Load --plan if present.
  3. Instantiate scanner; call Scan(repoRoot).
  4. Call facts.TruncateToSize if needed.
  5. Call renderer.WriteFactModel to save clarion-meta.json.
  6. Load LLM config; create adapter + pipeline.
  7. Call generator.GenerateSection("architecture", ...).
  8. Call renderer.WriteMarkdown("architecture.md", ...).
  9. Extract and write Mermaid from architecture.md.
  10. Print summary to stdout.

  MVP scope: only architecture.md and clarion-meta.json. Other sections (api, data-model, runbook) are stubs that print "not yet implemented".

4.9 Tests

  - TestGenerateSection: mock pipeline returning fixture Markdown; assert [INFERRED] markers applied correctly.
  - TestVerifySection: table-driven. Cases must include:
      (a) All claims matched with score >= 0.7: FailedClaims is empty, HighConfidence == 100%.
      (b) One claim matched with score 0.5: FailedClaims has 1 entry with Supported=false, Score=0.5; MediumConfidence > 0.
      (c) One claim with no FactModel match: FailedClaims has 1 entry with Supported=false, MatchedName="".
      (d) Empty Markdown input: all percentages are 0.0, no panic.
  - TestVerifyAll: golden fixture outputDir with 2 sections; one section clean, one
    with a known unsupported claim; assert allPassed=false and 1 FailedClaims entry.
  - TestPackMVP: end-to-end integration test scanning clarion's own source, generating architecture.md with mock LLM.
  - TestVerifyCommand: run verify on golden fixture; assert exit code 0 (all supported)
    and exit code 1 (unsupported claim present); assert stdout lists per-section
    confidence percentages in both cases.

Done criteria for Phase 4 (MVP):
  ✓ go test ./... passes.
  ✓ clarion pack enterprise on a Go repo produces clarion-meta.json and architecture.md.
  ✓ clarion verify on the generated output exits 0.
  ✓ Manually injecting an unsupported claim into architecture.md causes verify to exit 1.
  ✓ --json flag produces a single JSON object on stdout and nothing else.
  ✓ Binary runs in a CI environment (no TTY) without hanging.

⸻

Phase 5: Full Documentation Suite and Drift Detection

Goal: Complete all four documentation sections (SPEC.md §9), implement the drift command (SPEC.md §11), and the gen command (SPEC.md §5.2).

Entry condition: Phase 4 done (MVP stable).

5.1 Complete pack enterprise

  Extend pack.go to generate all four sections in sequence:
    architecture.md → api.md → data-model.md → runbook.md

  For each section after architecture:
  - Render the section template.
  - Run pipeline.Run.
  - Apply [INFERRED] markers.
  - Write the Markdown file.
  - Extract and write any embedded Mermaid diagram.

  Three-stage pipeline per section (SPEC.md §12.4):
  - Stage 1 (optional summarize): if FactModel is large, run a cheap
    summarization pass to compress it before the generation call. Skip
    if FactModel fits in 200KB without truncation.
  - Stage 2 (generate): main generation call.
  - Stage 3 (verify critique): ask LLM to self-critique the output for
    fabricated claims. Strip any sentence flagged as fabricated.

  Output files added in this phase:
    docs/api.md
    docs/data-model.md
    docs/runbook.md
    docs/diagrams/component.mmd
    docs/diagrams/sequence.mmd
    docs/diagrams/deployment.mmd

  Mermaid generation:
  - component.mmd: extracted from architecture.md.
  - sequence.mmd: extracted from architecture.md (second mermaid block, if present).
  - deployment.mmd: generated by a dedicated deployment template that
    lists entrypoints, external integrations, and datastores.

5.2 gen Command

File: /internal/cli/gen.go

  Usage: clarion gen <section>
  Supported sections: architecture, api, data-model, runbook.

  Pre-check: if clarion-meta.json absent in --output: Fatalf.

  Sequence:
  1. Load clarion-meta.json.
  2. Load LLM config.
  3. Call generator.GenerateSection for the specified section.
  4. Write the Markdown file (overwrite existing).
  5. Extract and write Mermaid if present.
  6. Does NOT re-scan the repo or update clarion-meta.json.

5.3 Drift Detection

File: /internal/drift/drift.go

  type DriftReport struct {
      GeneratedAt      time.Time     `json:"generated_at"`
      PreviousSnapshot time.Time     `json:"previous_snapshot"`
      DriftFraction    float64       `json:"drift_fraction"`
      Threshold        float64       `json:"threshold"`
      ExceededThreshold bool         `json:"exceeded_threshold"`
      Added            []DriftEntry  `json:"added"`
      Removed          []DriftEntry  `json:"removed"`
      Modified         []DriftEntry  `json:"modified"`
      Skipped          []string      `json:"skipped,omitempty"`
  }

  type DriftEntry struct {
      Type string `json:"type"`   // "component", "api", "datastore", etc.
      Name string `json:"name"`
      Change string `json:"change"` // "added", "removed", "modified"
  }

  func Compare(previous, current *facts.FactModel) DriftReport
    Implements the drift fraction formula from SPEC.md §11 exactly.
    Matching key: Name + collection type (e.g. APIEndpoint matched only against APIEndpoints).
    Modified detection: SourceFiles changed, or LineRanges changed,
                        or ConfidenceScore delta > 0.1.

  func (r DriftReport) Markdown() string
    Renders a human-readable drift-report.md.

5.4 drift Command

File: /internal/cli/drift.go

  Pre-check: clarion-meta.json absent → Fatalf with message from SPEC.md §5.

  Sequence:
  1. Load previous clarion-meta.json from --output.
  2. Validate --drift-threshold in [0.0, 1.0]; fatal if outside range.
  3. Re-scan the repo to build current FactModel.
  4. Call drift.Compare(previous, current).
  5. Write drift-report.md and drift-report.json to --output.
  6. Print DriftFraction and entry counts to stdout.
  7. If DriftFraction > threshold: os.Exit(ExitFailure).

5.5 Failure Mode Tests

  All failure modes from SPEC.md §11 must have explicit test cases:
  - Missing clarion-meta.json → exit 2, stderr contains
    "clarion-meta.json not found. Run clarion pack enterprise to generate an initial snapshot."
  - Malformed clarion-meta.json (invalid JSON) → exit 2, stderr contains "clarion-meta.json".
  - FactModel entry missing Name field → entry skipped, warning on stderr, entry listed
    in DriftReport.Skipped, comparison continues.
  - --drift-threshold 1.5 → exit 2, stderr contains "drift-threshold must be in [0.0, 1.0]".
  - drift_fraction > threshold → exit 1, stdout contains DriftFraction value.
  - drift_fraction == 0 on first run (total_entries == 0) → exit 0.

5.6 Tests

  - TestCompare: table-driven, covering add/remove/modify/no-change scenarios.
  - TestDriftFraction: verify formula with known inputs.
  - TestDriftMarkdown: golden file test for drift-report.md format.
  - TestGenCommand: mock LLM, assert single section regenerated without touching clarion-meta.json.
  - TestFullPack: mock LLM, assert all 4 .md and 3 .mmd files written.

Done criteria for Phase 5:
  ✓ go test ./... passes with zero failures.
  ✓ clarion pack enterprise produces all 7 output files:
      docs/architecture.md, docs/api.md, docs/data-model.md, docs/runbook.md,
      docs/diagrams/component.mmd, docs/diagrams/sequence.mmd,
      docs/diagrams/deployment.mmd (plus clarion-meta.json).
  ✓ clarion gen api regenerates only api.md; clarion-meta.json mtime is unchanged.
  ✓ clarion gen with an unsupported section name exits 2 with a usage error.
  ✓ clarion drift on an unchanged repo exits 0 with DriftFraction=0.0.
  ✓ clarion drift after adding an API endpoint exits 1 (default threshold 0.0).
  ✓ clarion drift --drift-threshold 0.5 exits 0 when drift fraction is below 0.5.
  ✓ TestDriftFraction: drift fraction formula matches SPEC.md §11 for add/remove/modify cases.
  ✓ All 6 failure modes from SPEC.md §11 produce correct exit codes (tested in 5.5).
  ✓ Three-stage pipeline (summarize → generate → verify) logs per-stage token counts.

⸻

Phase 6: Hardening, Observability, and CI Readiness

Goal: Reach production quality: performance validation, observability, golden file coverage, full CI pipeline. No new features.

Entry condition: Phase 5 done.

6.1 --emit-metrics Flag

  Add global flag --emit-metrics (bool, default false).
  When set, after command completion print to stdout:

    {
      "tokens_used": <int>,
      "estimated_cost": <float>,
      "duration_ms": <int>,
      "verification_failures": <int>
    }

  JSON format per SPEC.md §18.
  estimated_cost = tokens_used / 1000 * 0.01 (conservative estimate, documented as approximate).

6.2 --json Mode Telemetry

  When --json is enabled on pack enterprise, emit a single JSON result object:
    {
      "output_files": ["docs/architecture.md", ...],
      "tokens_used": <int>,
      "estimated_cost": <float>,
      "duration_ms": <int>,
      "verification_failures": 0,
      "fact_model_path": "docs/clarion-meta.json"
    }

6.3 Golden File Tests

  Directory: /internal/testdata/golden/

  Golden files for:
  - architecture.md (generated from a fixture FactModel)
  - api.md
  - data-model.md
  - runbook.md
  - component.mmd
  - drift-report.md

  Use the standard Go pattern:
    if *update { writeGolden(...) } else { assertMatchesGolden(...) }
  Run with: go test ./... -update to regenerate golden files.

6.4 Performance Validation

  File: /internal/scanner/bench_test.go

  BenchmarkScanLargeRepo: generates a synthetic 200k LOC Go repo (1000 files × 200 lines)
  in a temp directory and benchmarks Scan time.
  Assert: p99 < 10 seconds (enforced with t.Fatal if exceeded).

6.5 Security Hardening

  - Audit all os.Exec calls: there must be none (SPEC.md §17).
  - Audit all LLM prompts: assert no raw file content is included (only FactModel JSON, SPEC.md text, PLAN.md text).
  - Add a TestNoRawRepoInPrompt test that inspects the prompt passed to the mock LLM adapter and asserts no Go source code is present.
  - CLARION_LLM_API_KEY must never appear in any log line, error message, --debug
    dump, or --json output. The Config struct must store the key in a field tagged
    `json:"-"` and the Logf/Warnf helpers must redact it if it ever appears in a
    format string. Add TestAPIKeyNotLogged: run a pack with a known fake API key
    value and assert the key string does not appear anywhere on stderr or stdout.

6.6 CI Pipeline

  File: .github/workflows/ci.yml (or equivalent)

  Stages:
  1. lint      — golangci-lint run ./...
  2. test      — go test -race -count=1 ./...
  3. build     — go build -ldflags "..." ./cmd/clarion
  4. binary    — ./bin/clarion version (smoke test)

  Run on: push to main and all pull requests.
  Go version matrix: 1.22, 1.23.

6.7 Release Build

  Makefile target: release
  Produces static binaries for:
    linux/amd64
    linux/arm64
    darwin/amd64
    darwin/arm64
    windows/amd64

  Using: CGO_ENABLED=0 GOOS=<os> GOARCH=<arch> go build -ldflags "..."

Done criteria for Phase 6 (v1 complete):
  ✓ go test -race ./... passes with zero race conditions and zero data races.
  ✓ Golden files are committed; CI step `go test ./... -golden-check` fails if
    any golden file is stale (i.e., regenerating would produce a diff).
  ✓ BenchmarkScanLargeRepo p99 < 10s on the reference hardware defined in §2.8.
  ✓ TestNoRawRepoInPrompt passes: no Go source literals appear in any LLM prompt.
  ✓ No os.Exec or os.StartProcess calls exist anywhere (enforced by grep in CI).
  ✓ clarion pack enterprise --emit-metrics outputs valid JSON matching the schema
    in SPEC.md §18, with tokens_used > 0 and duration_ms > 0.
  ✓ clarion pack enterprise --json outputs a single valid JSON object and nothing
    else on stdout (validated by `clarion pack ... --json | jq .` in CI smoke test).
  ✓ CI pipeline (lint → test → build → smoke) passes on a clean checkout for
    both Go 1.22 and Go 1.23.
  ✓ release target produces 5 static binaries; each runs `clarion version` without
    error on its target platform.
  ✓ CLARION_LLM_API_KEY value never appears in any log output, --debug dump,
    or error message (TestAPIKeyNotLogged enforces this).

⸻

Dependency Graph

  Phase 0 ──► Phase 1 ──► Phase 2 ──┐
                 │                   ├──► Phase 4 ──► Phase 5 ──► Phase 6
                 └──► Phase 3 ───────┘

  Phases 2 and 3 are independent of each other and can be developed in parallel
  after Phase 1 is complete.

⸻

Key Invariants (enforced throughout all phases)

  • No circular imports between /internal packages.
  • No os.Exec or os.StartProcess anywhere in the codebase.
  • LLM prompts never contain raw repository file contents.
  • ConfidenceScore values are set exactly once by the scanner; never mutated later.
  • All os.Exit calls use ExitSuccess, ExitFailure, or ExitFatal constants.
  • All user-facing error messages follow the exact wording specified in SPEC.md §5.
  • Every new package must have at least one test file before Phase 4 begins.
  • CLARION_LLM_API_KEY must never appear in any log, error message, or output stream.
