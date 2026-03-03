# Clarion

> **Make systems legible.**

Clarion is a Go CLI tool that generates evidence-backed documentation by scanning a repository, building a structured Fact Model, and using an LLM to produce documentation from that model — never from raw source code.

Every claim in the output references the source file and line range that supports it. If Clarion cannot prove a claim, it says so.

---

## Contents

- [How it works](#how-it-works)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Commands](#commands)
- [Global flags](#global-flags)
- [LLM configuration](#llm-configuration)
- [Output files](#output-files)
- [Evidence and confidence scores](#evidence-and-confidence-scores)
- [Exit codes](#exit-codes)
- [CI integration](#ci-integration)
- [Development](#development)
- [Security](#security)
- [License](#license)

---

## How it works

```
Repository source code
        │
        ▼
  ┌─────────────┐
  │   Scanner   │  go/parser, go/ast — extracts metadata only, never file content
  └──────┬──────┘
         │  FactModel (components, APIs, datastores, jobs, config, integrations)
         ▼
  ┌─────────────┐
  │ Fact Model  │  Serialised to clarion-meta.json — the authoritative evidence store
  └──────┬──────┘
         │  FactModel JSON + SPEC.md + PLAN.md
         ▼
  ┌─────────────┐
  │  Generator  │  LLM produces documentation grounded in the Fact Model
  └──────┬──────┘
         │  Markdown + Mermaid diagrams
         ▼
  ┌─────────────┐
  │  Verifier   │  Every claim cross-referenced against the Fact Model
  └─────────────┘
```

The LLM operates only on the Fact Model JSON, `SPEC.md`, and `PLAN.md` — never on raw repository files.

---

## Installation

**Build from source** (requires Go 1.22+):

```bash
git clone https://github.com/clarion-dev/clarion
cd clarion
make build
# binary at bin/clarion
```

**Cross-platform static binaries:**

```bash
make release
# produces bin/clarion-{linux,darwin}-{amd64,arm64} and bin/clarion-windows-amd64.exe
```

---

## Quick start

### Initial documentation generation

```bash
# Set LLM credentials
export CLARION_LLM_PROVIDER=openai
export CLARION_LLM_MODEL=gpt-4o
export CLARION_LLM_API_KEY=sk-...

# Scan the repo, build the Fact Model, generate all documentation
clarion pack enterprise --spec SPEC.md --plan PLAN.md --output docs/

# Verify all generated claims are evidence-backed
clarion verify --output docs/
```

### After code changes

```bash
# Check how much the codebase has drifted from the last snapshot
clarion drift --output docs/ --drift-threshold 0.20

# Regenerate if drift is within acceptable bounds
clarion pack enterprise --spec SPEC.md --plan PLAN.md --output docs/
clarion verify --output docs/
```

### Regenerate a single section

```bash
clarion gen architecture --output docs/
```

---

## Commands

### `clarion pack enterprise`

Scans the repository, builds the Fact Model, and generates all four documentation sections.

```
clarion pack enterprise [flags]
```

**Output files** (written to `--output`):

| File | Description |
|------|-------------|
| `architecture.md` | System overview, components, data flow, trust boundaries |
| `api.md` | API endpoints, methods, routes, auth patterns |
| `data-model.md` | Datastores, schemas, relationships, PII flags |
| `runbook.md` | Startup, environment variables, health checks, known unknowns |
| `diagrams/component.mmd` | Mermaid component diagram |
| `clarion-meta.json` | The Fact Model — authoritative evidence store |

**JSON mode** (`--json`): emits a single result object to stdout:

```json
{
  "output_files": ["docs/architecture.md", "..."],
  "tokens_used": 12450,
  "estimated_cost": 0.1245,
  "duration_ms": 8320,
  "verification_failures": 0,
  "fact_model_path": "docs/clarion-meta.json"
}
```

---

### `clarion gen <section>`

Regenerates a single documentation section from an existing `clarion-meta.json` without re-scanning the repository.

```
clarion gen architecture|api|data-model|runbook [flags]
```

Requires an existing `clarion-meta.json` in `--output`. Use after increasing `CLARION_LLM_TOKEN_BUDGET` to complete a partial run, or to refresh a single section without a full re-scan.

---

### `clarion verify`

Validates every claim in the generated documentation against the Fact Model.

```
clarion verify [flags]
```

For each claim extracted from the Markdown files, the verifier looks up a matching entry in `clarion-meta.json` by name. A claim passes if the matched entry has `ConfidenceScore >= 0.7`. The command exits `1` if any claim fails.

Requires both existing documentation files and `clarion-meta.json` in `--output`.

---

### `clarion drift`

Compares the current repository state against the previous `clarion-meta.json` snapshot and generates a drift report.

```
clarion drift [--drift-threshold <float>] [flags]
```

**Output files:**

| File | Description |
|------|-------------|
| `drift-report.json` | Structured drift report |
| `drift-report.md` | Human-readable drift summary |

**Drift fraction formula:**

```
drift_fraction = (added + removed + modified) / total_previous_entries
```

Where *modified* means `SourceFiles` changed, `LineRanges` changed, or `ConfidenceScore` delta > 0.1.

| Flag | Default | Description |
|------|---------|-------------|
| `--drift-threshold` | `0.0` | Exit `1` if drift fraction exceeds this value. `0.0` means any drift fails. |

**Example — allow up to 20% drift:**

```bash
clarion drift --drift-threshold 0.20
```

---

### `clarion version`

```
clarion version
# clarion v0.1.0  commit:abc1234  built:2024-01-15T10:30:00Z
```

---

## Global flags

All commands accept these persistent flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--spec <path>` | `./SPEC.md` | Path to SPEC.md. Must exist and be readable. |
| `--plan <path>` | `./PLAN.md` | Path to PLAN.md. Optional — omitted if absent. |
| `--output <dir>` | `./docs` | Directory for generated output. Created if absent. |
| `--json` | `false` | Emit structured JSON to stdout. Mutually exclusive with `--verbose`. |
| `--verbose` | `false` | Print step-by-step processing details to stderr. |
| `--emit-metrics` | `false` | Print token usage and cost metrics after completion. |

**Metrics output** (`--emit-metrics`):

```json
{
  "tokens_used": 12450,
  "estimated_cost": 0.1245,
  "duration_ms": 8320,
  "verification_failures": 0
}
```

---

## LLM configuration

All configuration is via environment variables. No credentials are written to disk.

| Variable | Required | Description |
|----------|----------|-------------|
| `CLARION_LLM_PROVIDER` | Yes | `openai` or `anthropic` |
| `CLARION_LLM_MODEL` | Yes | Model name (e.g. `gpt-4o`, `claude-opus-4-6`) |
| `CLARION_LLM_API_KEY` | Yes | API key for the selected provider |
| `CLARION_LLM_TOKEN_BUDGET` | No | Maximum tokens per run. Default: `100000` |

**OpenAI example:**

```bash
export CLARION_LLM_PROVIDER=openai
export CLARION_LLM_MODEL=gpt-4o
export CLARION_LLM_API_KEY=sk-...
```

**Anthropic example:**

```bash
export CLARION_LLM_PROVIDER=anthropic
export CLARION_LLM_MODEL=claude-opus-4-6
export CLARION_LLM_API_KEY=sk-ant-...
```

### Token budget

The token budget is enforced cumulatively across all pipeline stages. If the budget is exhausted mid-run, completed sections are written to disk and Clarion exits `1`. Use `clarion gen <section>` to regenerate the skipped sections after increasing `CLARION_LLM_TOKEN_BUDGET`.

The Fact Model is capped at 200 KB per LLM call. If the serialized model exceeds this, entries with the lowest confidence scores are dropped and a warning is emitted to stderr.

### Error handling

| Condition | Behaviour |
|-----------|-----------|
| Transient error (rate limit, 5xx) | Retry once after 2 seconds |
| Second consecutive failure | Exit `2` with provider, model, and HTTP status |
| Authentication error (401/403) | Exit `2` immediately, no retry |
| Malformed API response | Exit `2` with raw response excerpt (max 200 chars) |

---

## Output files

A complete `clarion pack enterprise` run produces:

```
docs/
├── architecture.md        # System overview and component breakdown
├── api.md                 # API reference
├── data-model.md          # Datastores, schemas, ER diagram
├── runbook.md             # Operational procedures
├── diagrams/
│   └── component.mmd      # Mermaid component diagram
├── clarion-meta.json      # Fact Model snapshot (evidence store)
├── drift-report.json      # Written by `clarion drift`
└── drift-report.md        # Written by `clarion drift`
```

---

## Evidence and confidence scores

Every entry in the Fact Model carries a confidence score assigned during scanning:

| Score | Tier | Meaning |
|-------|------|---------|
| `0.9` | Direct | Symbol explicitly registered or declared (e.g., HTTP handler registered on a router by name) |
| `0.7` | Indirect | Symbol matches a known pattern without explicit registration (e.g., function signature matches handler interface) |
| `0.5` | Inferred | Named convention or partial structural match; included with `[INFERRED]` marker |
| `0.2` | Speculative | Keyword or comment match only; omitted from generated documentation |

Claims with `ConfidenceScore` in `[0.4, 0.7)` or with `Inferred: true` are rendered in documentation with an `[INFERRED]` suffix. Claims below `0.4` are omitted entirely and logged as skipped evidence.

`clarion verify` passes only when every claim maps to a Fact Model entry with `ConfidenceScore >= 0.7`.

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Verify failure or drift threshold exceeded — the tool ran correctly but a post-condition was not met |
| `2` | Fatal error — bad configuration, I/O failure, LLM error, etc. |

---

## CI integration

```yaml
# .github/workflows/docs.yml
- name: Generate documentation
  env:
    CLARION_LLM_PROVIDER: openai
    CLARION_LLM_MODEL: gpt-4o
    CLARION_LLM_API_KEY: ${{ secrets.CLARION_LLM_API_KEY }}
  run: |
    clarion pack enterprise --spec SPEC.md --plan PLAN.md --output docs/
    clarion verify --output docs/

- name: Check for documentation drift
  run: clarion drift --output docs/ --drift-threshold 0.20
```

**Concurrent execution:** do not run multiple Clarion commands against the same `--output` directory simultaneously. Use distinct output directories for parallel CI jobs.

---

## Development

### Prerequisites

- Go 1.22+
- `golangci-lint` (for linting)

### Common commands

```bash
make build          # Build binary to bin/clarion
make test           # Run all tests
make lint           # Run golangci-lint
make golden         # Regenerate golden test fixtures
make release        # Build static binaries for all platforms
make clean          # Remove bin/
```

**Run a single package's tests:**

```bash
go test ./internal/scanner/...
go test -run TestScanGoRepository ./internal/scanner/
```

**Run with race detector:**

```bash
go test -race ./...
```

**Run the benchmark:**

```bash
go test -bench=BenchmarkScanLargeRepo -benchtime=1x ./internal/scanner/
```

### Project layout

```
cmd/clarion/           Binary entrypoint
internal/
  scanner/             AST-based repo analysis (go/parser, go/ast)
  facts/               FactModel types and clarion-meta.json serialization
  llm/                 OpenAI and Anthropic adapters, pipeline, budget tracker
  generator/           LLM-driven documentation generation, prompt templates
  render/              Markdown, Mermaid, and JSON output rendering
  verify/              Claim extraction and FactModel cross-reference
  drift/               Drift detection between FactModel snapshots
  cli/                 Cobra command wiring
  testdata/            Golden file tests
specs/
  SPEC.md              Full technical specification
  PLAN.md              Phased implementation plan
```

### Adding a new documentation section

1. Add a prompt template to `internal/generator/templates/<section>.tmpl`
2. Register the section name in `internal/cli/pack.go` and `internal/cli/gen.go`
3. Add a `sectionMermaidFile` entry in `internal/cli/pack.go` if the section produces a diagram
4. Add a golden file test in `internal/testdata/golden_test.go`
5. Run `make golden` to generate the fixture

---

## Security

- **No code execution.** Clarion never runs repository code or invokes arbitrary shell commands.
- **No raw source in LLM prompts.** The scanner extracts metadata only (names, file paths, confidence scores — never file content). The LLM receives only Fact Model JSON, `SPEC.md`, and `PLAN.md`. This is enforced at runtime by `pipeline.Run` and verified by `TestNoRawRepoInPrompt`.
- **API key protection.** `CLARION_LLM_API_KEY` is tagged `json:"-"` and never appears in log output, `--json` responses, or error messages.
- **Respects `.gitignore`.** The scanner skips files and directories excluded by `.gitignore` by default.
- **Advisory output lock.** Commands acquire a per-output-directory lock file (`.clarion.lock`) to prevent accidental concurrent writes.

---

## License

MIT — see [LICENSE](LICENSE).

---

*Documentation must be derived, not imagined. If the tool cannot prove a claim, it must say so.*
