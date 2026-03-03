# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Status

Clarion is currently in the **specification phase**. The full technical design is in `specs/SPEC.md`. No implementation exists yet.

## What Clarion Is

A Go 1.22+ CLI tool (`clarion`) that generates evidence-backed documentation by scanning a repository, building a structured Fact Model, and using LLMs to produce documentation from that model — never from raw source. Core philosophy: *documentation must be derived, not imagined.*

## Planned Build & Run Commands

Once implemented (entry point at `/cmd/clarion/main.go`):

```bash
go build ./cmd/clarion/...          # Build binary
go test ./...                       # Run all tests
go test ./internal/scanner/...      # Run single package tests
go vet ./...                        # Vet

clarion pack enterprise --spec SPEC.md --plan PLAN.md
clarion gen architecture
clarion drift --drift-threshold 0.75
clarion verify
```

Exit codes: `0` = success, `1` = verification/drift failure, `2` = fatal error.

All commands support `--spec`, `--plan`, `--output`, `--json`, `--verbose` flags.

## Module Structure

```
/cmd/clarion/          # Binary entrypoint
/internal/scanner/     # Repo analysis (language detection, entrypoints, deps, APIs, DBs, jobs, config)
/internal/facts/       # FactModel construction and serialization
/internal/generator/   # LLM-driven documentation generation
/internal/verify/      # Claim verification against FactModel
/internal/drift/       # Drift detection between FactModel snapshots
/internal/llm/         # OpenAI and Anthropic client abstractions
/internal/render/      # Markdown, Mermaid, JSON output rendering
/internal/cli/         # CLI wiring
```

No circular dependencies between packages.

## Five-Layer Architecture

1. **Scanner** — Parses repo using `go/parser`, `go/ast`, `go/token`, `go/types`. Detects HTTP handlers, router registrations, struct tags, DB patterns, frameworks (gin, chi, echo). Must not use LLM for structural extraction.
2. **Fact Model Builder** — Constructs the canonical `FactModel` struct (see below). Every claim must carry `SourceFiles`, `LineRanges`, and `ConfidenceScore`. Serialized to `docs/clarion-meta.json` (the authoritative evidence store).
3. **Documentation Generator** — LLMs operate only on `FactModel` JSON + `SPEC.md` + `PLAN.md` contents. Never raw repository text.
4. **Verification Engine** — Parses generated docs, extracts claims, cross-references against FactModel, flags unsupported claims. Assigns high/medium/low confidence per section.
5. **Output Renderer** — Writes Markdown, Mermaid diagrams (`.mmd`), and JSON.

## Core Data Model

```go
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
```

Every entry includes: `Name`, `Description`, `SourceFiles []string`, `LineRanges []Range`, `ConfidenceScore float64`, `Inferred bool`.

## LLM Integration

Supports OpenAI and Anthropic, configured via environment variables. Three-stage pipeline: summarization (optional small model) → documentation generation → verification critique. Must log tokens used, cost estimate, and duration. Use `--json` for structured telemetry.

## Testing Strategy

- Table-driven unit tests for AST parsing, FactModel generation, and drift comparison
- Golden file tests for Markdown and Mermaid output
- Mock LLM client for deterministic testing (same inputs → identical outputs at temperature 0)

## Key Constraints

- Single static binary, no heavy runtime dependencies
- No code execution or arbitrary shell invocation
- Do not transmit raw repository to LLM unless explicitly enabled
- Respect `.gitignore` by default
- Repo scanning must complete in under 10 seconds for medium projects
- Must handle repos up to 200k LOC

## MVP Definition of Done

Parse Go repo → build FactModel → generate `architecture.md` → generate `clarion-meta.json` → support `verify` command → run in CI without interaction.

No feature expansion until these are stable.
