SPEC.md

Clarion

Make systems legible.

⸻

1. Overview

Clarion is a Go-based CLI tool that generates evidence-backed documentation from:
	•	Source code repositories
	•	SPEC.md
	•	PLAN.md

Clarion is designed to operate as a composable step inside AI coding workflows and CI pipelines.

Clarion does not generate arbitrary documentation. It generates a fixed set of documentation artifacts (architecture, API reference, data model, runbook, and Mermaid diagrams) with traceable evidence linking every claim to a source file and line range.

⸻

2. Goals
	1.	Generate structured documentation from code and specifications.
	2.	Ensure all documentation claims are evidence-backed. This requirement is verified automatically by `clarion verify`. The verification algorithm is deterministic: for each claim extracted from the documentation, the tool looks up a matching entry in clarion-meta.json by Name; if no match is found, or if the matched entry has a ConfidenceScore < 0.7, the claim is reported as unsupported and the command exits 1. Verification passes (exit 0) only when every claim maps to a FactModel entry with ConfidenceScore >= 0.7. No human judgment is required or permitted in this check.
	3.	Provide deterministic, reproducible CLI output.
	4.	Integrate cleanly into AI agent workflows.
	5.	Support drift detection between code and documentation.

⸻

3. Non-Goals

Clarion v1 will NOT:
	•	Replace OpenAPI generators.
	•	Replace godoc/typedoc tools.
	•	Automatically rewrite README.md.
	•	Generate marketing documentation.
	•	Infer business logic not present in source or spec.

⸻

4. Technical Constraints
	•	Language: Go 1.22+
	•	No heavy runtime dependencies.
	•	Must compile to a single static binary.
	•	CLI-first architecture.
	•	No required external services beyond optional LLM APIs.
	•	Must work in non-interactive CI environments.

⸻

5. CLI Interface

Binary name:
clarion

All commands must support:

Flag behavior:
	•	–spec <path>    Path to SPEC.md. Default: ./SPEC.md. Must exist and be readable.
	•	–plan <path>    Path to PLAN.md. Default: ./PLAN.md. If absent, proceed without it.
	•	–output <dir>   Directory for generated output files. Default: ./docs. Created if absent.
	•	–json           Emit all output as structured JSON to stdout. Suppresses human-readable progress text. Mutually exclusive with –verbose.
	•	–verbose        Print step-by-step processing details to stderr. Default: false.

Output conventions:
	•	Progress messages and warnings: stderr.
	•	Generated documentation files: written to the –output directory.
	•	Summary result (human-readable): stdout on command completion, one line per output file written.
	•	If –json: stdout receives a single JSON object; no other stdout output is emitted.

Exit codes:
0 = success
1 = verification/drift failure
2 = fatal error

Command execution order:

Commands are independently executable. No command enforces a prerequisite at runtime, but the following constraints apply:
	•	verify requires existing documentation in the –output directory and a valid clarion-meta.json; if either is absent it exits code 2.
	•	drift requires an existing clarion-meta.json; if absent it exits code 2.
	•	gen <section> regenerates a single output file without updating clarion-meta.json.

Recommended sequence for initial generation:
  1. clarion pack enterprise   (scans repo, builds FactModel, generates all docs)
  2. clarion verify            (validates generated docs against FactModel)

Recommended sequence after code changes:
  1. clarion drift             (compare current repo to previous clarion-meta.json)
  2. clarion pack enterprise   (regenerate if drift is acceptable)
  3. clarion verify

Commands must not be run concurrently against the same –output directory. Concurrent execution against the same directory produces undefined behavior. For parallel CI jobs, use distinct –output values.

Out-of-order execution behavior:
	•	`clarion verify` with no documentation in –output: exit code 2, message "No documentation found in <dir>. Run clarion pack enterprise first."
	•	`clarion verify` with no clarion-meta.json in –output: exit code 2, message "clarion-meta.json not found in <dir>. Run clarion pack enterprise first."
	•	`clarion drift` with no clarion-meta.json in –output: exit code 2, message "clarion-meta.json not found. Run clarion pack enterprise to generate an initial snapshot."
	•	`clarion gen <section>` with no clarion-meta.json: exit code 2, message "clarion-meta.json not found. Run clarion pack enterprise first."

⸻

5.1 Command: pack enterprise

Generates:

docs/
architecture.md
api.md
data-model.md
runbook.md
diagrams/
component.mmd
sequence.mmd
deployment.mmd
clarion-meta.json

Usage:

clarion pack enterprise –spec SPEC.md –plan PLAN.md

⸻

5.2 Command: gen

Generates a single section.

Supported sections:
	•	architecture
	•	api
	•	data-model
	•	runbook

Example:

clarion gen architecture

⸻

5.3 Command: drift

Compares current repository state with previous clarion-meta.json.

Outputs:
	•	drift-report.md
	•	drift-report.json

Fails with exit code 1 if drift detected beyond threshold.

⸻

5.4 Command: verify

Re-validates existing documentation against current Fact Model.

Fails if unsupported claims detected.

⸻

6. System Architecture

Clarion is structured into five layers:
	1.	Repo Scanner
	2.	Fact Model Builder
	3.	Documentation Generator
	4.	Verification Engine
	5.	Output Renderer

Each layer must be modular and independently testable.

⸻

7. Repository Analysis Layer

7.1 Responsibilities
	•	Detect language(s)
	•	Identify entrypoints
	•	Parse dependency manifests
	•	Detect API frameworks
	•	Identify database usage
	•	Identify background jobs
	•	Identify config usage
	•	Identify external integrations

7.2 Go-Specific Parsing

Use:
	•	go/parser
	•	go/ast
	•	go/token
	•	go/types

Detect:
	•	http handlers
	•	router registrations
	•	struct tags (json, db)
	•	context usage
	•	database/sql usage
	•	popular frameworks (gin, chi, echo)

Must not use LLM for structural extraction.

⸻

8. Fact Model

Internal canonical structure:

type FactModel struct {
    Project      ProjectInfo
    Languages    []string
    Components   []Component
    APIs         []APIEndpoint
    Datastores   []Datastore
    Jobs         []BackgroundJob
    Integrations []ExternalIntegration
    Config       []ConfigVar
    Security     SecurityModel
}

Each entry must include:
	•	Name
	•	Description (optional)
	•	SourceFiles []string
	•	LineRanges []Range
	•	ConfidenceScore float64
	•	Inferred bool

This model is serialized to:

docs/clarion-meta.json

This is the authoritative evidence store.

⸻

8.1 Evidence-Backed Claims

A documentation claim is considered evidence-backed if and only if:
	•	It references at least one SourceFile and LineRange present in the FactModel.
	•	Its ConfidenceScore is >= 0.7.

Claims with ConfidenceScore in [0.4, 0.7) are permitted but must be rendered with an [INFERRED] marker in the output document.

Claims with ConfidenceScore < 0.4 must be omitted from generated documentation and logged as skipped evidence.

Claims with Inferred: true must be rendered with an [INFERRED] marker regardless of score.

The verify command fails (exit code 1) if any claim in the generated document cannot be matched to a FactModel entry.

ConfidenceScore assignment rules (applied during scanning):
	•	0.9: Direct structural evidence — a symbol is explicitly registered or declared (e.g., HTTP handler registered by name on a router).
	•	0.7: Indirect structural evidence — a symbol matches a known pattern without explicit registration (e.g., function signature matches a handler interface).
	•	0.5: Inferred from naming conventions or partial structural match (e.g., function named FooHandler with no router registration found).
	•	0.2: Speculative — keyword or comment match only, no AST-level proof.

Scanners must assign scores using these rules. Scores must not be manually adjusted after assignment.

⸻

9. Documentation Generation

LLMs must operate only on:
	•	FactModel JSON
	•	SPEC.md contents
	•	PLAN.md contents

Never raw repository text.

9.1 Architecture Document

Must include:
	•	System overview
	•	Component breakdown
	•	Data flow explanation
	•	External dependencies
	•	Trust boundaries
	•	Mermaid component diagram

Claims must reference evidence.

⸻

9.2 API Document

Must include:
	•	Endpoint list
	•	Method
	•	Route
	•	Auth pattern (if detectable)
	•	Error handling conventions

Must not fabricate undocumented endpoints.

⸻

9.3 Data Model Document

Must include:
	•	Table/entity summaries
	•	Field summaries
	•	Relationship notes
	•	PII heuristic detection

Must generate Mermaid ER diagram.

⸻

9.4 Runbook

Must include:
	•	Startup instructions
	•	Required environment variables
	•	External dependencies
	•	Health endpoints
	•	Logging strategy
	•	Known unknowns section

If uncertain, explicitly state UNKNOWN.

⸻

10. Verification Engine

Verification must:
	1.	Parse generated documentation.
	2.	Extract claims.
	3.	Cross-reference against FactModel.
	4.	Flag unsupported claims.

Each section must receive:
	•	High confidence %
	•	Medium confidence %
	•	Low confidence %

If unsupported claims exist:
	•	Mark section failed.
	•	Exit with code 1 if –verify enabled.

⸻

11. Drift Detection

Algorithm:
	1.	Load previous clarion-meta.json.
	2.	Regenerate FactModel.
	3.	Compare:
	•	Added/removed components
	•	API changes
	•	Datastore changes
	•	Config changes
	4.	Generate drift report.

Configurable:

–drift-threshold float   Fraction of changed entries that triggers exit code 1. Default: 0.0 (any drift fails).

Drift fraction is calculated as:
  drift_fraction = (added_entries + removed_entries + modified_entries) / total_entries_in_previous_FactModel

where:
  added_entries    = entries present in current FactModel but absent in previous (matched by Name + type).
  removed_entries  = entries present in previous FactModel but absent in current.
  modified_entries = entries present in both, but with changed SourceFiles, LineRanges, or ConfidenceScore delta > 0.1.
  total_entries    = count of all entries in the previous clarion-meta.json.

If total_entries == 0 (first run), drift_fraction = 0.0 and the command always exits 0.

Failure modes:
	•	If clarion-meta.json is missing or unreadable: emit error to stderr, exit code 2. Do not proceed.
	•	If FactModel regeneration fails (e.g. parse error): emit error to stderr, exit code 2. Do not produce a partial report.
	•	If a FactModel entry contains malformed data (missing Name or SourceFiles): skip that entry, log a warning, and continue comparison. Flag skipped entries in the drift report.
	•	If –drift-threshold is outside [0.0, 1.0]: emit error to stderr, exit code 2.

⸻

12. LLM Integration

Support:
	•	OpenAI
	•	Anthropic

12.1 Configuration

All configuration via environment variables:

	CLARION_LLM_PROVIDER   Required. One of: openai, anthropic.
	CLARION_LLM_MODEL      Required. Model name (e.g. gpt-4o, claude-opus-4-6).
	CLARION_LLM_API_KEY    Required. API key for the selected provider.
	CLARION_LLM_TOKEN_BUDGET  Optional. Max tokens to spend per run. Default: 100000.

Token budget enforcement:
	•	Token spend is tracked cumulatively across all pipeline stages (PromptTokens + CompletionTokens per call).
	•	Before Stage 1 begins, if CLARION_LLM_TOKEN_BUDGET is 0 or negative: emit error "CLARION_LLM_TOKEN_BUDGET must be > 0" to stderr and exit code 2 immediately. No output files are written.
	•	Before Stage 1 begins, if the estimated token cost of Stage 1 exceeds CLARION_LLM_TOKEN_BUDGET: emit "Token budget too small to begin. Increase CLARION_LLM_TOKEN_BUDGET." to stderr and exit code 2 immediately. No output files are written.
	•	Before each stage begins, the estimated token cost for that stage (prompt size in tokens) is compared against the remaining budget. If the remaining budget is insufficient for the next stage: skip that stage and all subsequent stages, write any completed output files, emit "Token budget exceeded: <used>/<budget> tokens. Remaining stages skipped." to stderr, and exit code 1.
	•	After each LLM call completes, the running total is updated with the actual PromptTokens + CompletionTokens returned.
	•	Any output files produced by completed stages are written to disk and are considered valid partial output. These files are safe to use independently; the user may run `clarion gen <section>` to regenerate only the skipped sections after increasing CLARION_LLM_TOKEN_BUDGET.

If required variables are unset at startup, exit code 2 with a descriptive error message.

12.2 Interface Contract

Input to every LLM call (internal Go struct, translated to the provider's native API format by a provider adapter):

type LLMRequest struct {
    Model       string   // from CLARION_LLM_MODEL
    Temperature float64  // always 0.0
    Prompt      string   // template-rendered string containing FactModel JSON + SPEC.md + optional PLAN.md
    MaxTokens   int      // from CLARION_LLM_TOKEN_BUDGET remainder
}

All provider adapters must implement this Go interface:

type ProviderAdapter interface {
    // Name returns the provider identifier (e.g. "openai", "anthropic").
    Name() string

    // Validate checks that the adapter is correctly configured (API key present,
    // model name non-empty). Called at startup; returns an error if misconfigured.
    Validate() error

    // Call sends a single LLM request and returns the response or an error.
    // Implementations must apply the retry and error-handling rules in Section 12.3.
    Call(ctx context.Context, req LLMRequest) (LLMResponse, error)
}

Provider adapters must translate LLMRequest to:
	•	OpenAI: POST /v1/chat/completions with model, temperature, max_tokens, and a single user message containing Prompt.
	•	Anthropic: POST /v1/messages with model, temperature, max_tokens, and a single user message containing Prompt.

FactModel JSON payload within Prompt: max 200KB. If the serialized FactModel exceeds 200KB, drop entries with the lowest ConfidenceScore until it fits, and log a warning to stderr listing how many entries were dropped.

Output from every LLM call:

type LLMResponse struct {
    Text             string  // generated content returned by the model
    PromptTokens     int     // tokens consumed by the prompt
    CompletionTokens int     // tokens in the model's response
    LatencyMS        int     // wall-clock time from request to first response byte
    ModelID          string  // model identifier as echoed by the provider
}

Provider adapters must populate all fields of LLMResponse from the provider's response payload. If a field is absent in the response, it must be set to 0 or empty string (never omitted).

12.3 Error Handling

	•	On transient API error (rate limit, timeout, 5xx): retry once after 2 seconds.
	•	On second consecutive failure: exit code 2 with error message including provider, model, and HTTP status.
	•	On authentication error (401/403): exit code 2 immediately with no retry.
	•	On token budget exceeded mid-run: emit a warning, write partial output, exit code 1.
	•	On malformed/unparseable API response: exit code 2 with the raw response excerpt (max 200 chars) in the error message.
	•	On any other unexpected error (network failure, context timeout, unknown exception): emit the error message and stack trace to stderr, exit code 2.

12.4 Pipeline Stages

Stage 1: Summarization (optional, use a smaller/cheaper model if desired)
Stage 2: Documentation generation
Stage 3: Verification critique

Must log per-stage:
	•	tokens used
	•	cost estimate
	•	duration

If –json enabled, emit structured telemetry.

⸻

13. Project Structure (Go)

Recommended module layout:

/cmd/clarion
main.go

/internal/scanner
/internal/facts
/internal/generator
/internal/verify
/internal/drift
/internal/llm
/internal/render
/internal/cli

No circular dependencies.

⸻

14. Testing Strategy
	•	Unit tests for:
	•	AST parsing
	•	Fact model generation
	•	Drift comparison
	•	Golden file tests for:
	•	Markdown output
	•	Mermaid output
	•	Mock LLM client for deterministic tests.

Use table-driven tests.

⸻

15. Determinism Requirements

Given:
	•	Same repo
	•	Same SPEC.md
	•	Same PLAN.md
	•	Same model + temperature 0

Clarion must produce identical output.

⸻

16. Performance Requirements
	•	Must handle repositories up to 200k LOC.
	•	Repo scanning must complete in under 10 seconds for medium projects.
	•	LLM usage must be bounded by:
	•	FactModel size limits
	•	Token budget caps

⸻

17. Security Considerations
	•	No code execution.
	•	No arbitrary shell invocation.
	•	Do not transmit raw repository unless explicitly enabled.
	•	Respect .gitignore by default.

⸻

18. Observability

If –emit-metrics:

Output JSON:

{
“tokens_used”: 0,
“estimated_cost”: 0.00,
“duration_ms”: 0,
“verification_failures”: 0
}

⸻

19. Versioning

Clarion must embed version info:

clarion version

Output:

clarion v0.1.0
commit:
built:

⸻

20. Definition of Done (MVP)

MVP must:
	•	Parse Go repository.
	•	Build FactModel.
	•	Generate architecture.md.
	•	Generate clarion-meta.json.
	•	Support verify command.
	•	Run in CI without interaction.

No feature expansion until these are stable.

⸻

Clarion’s core philosophy:

Documentation must be derived, not imagined.
If the tool cannot prove a claim, it must say so.
