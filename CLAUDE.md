# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build          # Build binary → bin/clarion
make test           # go test ./... ./internal/testdata/
make lint           # golangci-lint run ./...
make golden         # Regenerate golden test fixtures (-update)
make release        # Cross-platform static binaries (5 targets)
make clean          # Remove bin/

go test ./internal/scanner/...                    # Single package
go test -run TestScanGoRepository ./internal/scanner/
go test -race -count=1 ./...                      # Race detector
go test -bench=BenchmarkScanLargeRepo ./internal/scanner/

prism review staged --provider openai --model gpt-4o --format markdown
```

## Architecture (5 layers)

```
Scanner → Fact Model → LLM Integration → Generator → Verifier/Renderer
```

| Package | Responsibility |
|---------|----------------|
| `internal/scanner/` | AST-based repo analysis (`go/parser`, `go/ast`). No file content — metadata only. |
| `internal/facts/` | `FactModel` types, `clarion-meta.json` serialization, `TruncateToSize` |
| `internal/llm/` | Config, `BudgetTracker`, `Pipeline`, OpenAI/Anthropic adapters, `MockAdapter`, `enforceSpec9` guard |
| `internal/generator/` | `templateData` (FactModel JSON + spec + plan only), `GenerateSection`, `ApplyInferredMarkers` |
| `internal/render/` | `WriteMarkdown`, `WriteMermaid`, `WriteFactModel`, `WriteJSON`, `ExtractMermaid` |
| `internal/verify/` | `VerifySection`, `VerifyAll` — cross-references claims against FactModel |
| `internal/drift/` | `Compare`, `DriftReport`, `Markdown()` |
| `internal/cli/` | Cobra commands: `pack enterprise`, `gen`, `verify`, `drift`, `version` |

## Key patterns

- **ErrVerifyFailed / ErrDriftExceeded**: sentinel errors returned from `RunE` so deferred cleanup runs; `main.go` maps them to exit 1 vs exit 2.
- **AcquireLock**: advisory `.clarion.lock` per output dir; all errors after `AcquireLock` must use `return fmt.Errorf` (not `Fatalf`/`os.Exit`).
- **Confidence tiers**: Direct=0.9, Indirect=0.7, Inferred=0.5, Speculative=0.2. Scores never manually adjusted after assignment.
- **`[INFERRED]` marker**: applied by `ApplyInferredMarkers` for entries with score in [0.4, 0.7) or `Inferred: true`; entries below 0.4 are omitted.
- **SPEC.md §9 enforcement**: `enforceSpec9` in `internal/llm/spec9_guard.go` rejects prompts starting with a bare Go package declaration; `templateData` ensures only FactModel JSON + spec + plan enter prompts.
- **Stale LSP diagnostics**: always verify with `go build ./...` before acting on IDE "undefined" errors — new files in a package often cause stale LSP state.
- **`--emit-metrics`**: JSON telemetry after command completion (`tokens_used`, `estimated_cost`, `duration_ms`, `verification_failures`).

## Module

`github.com/clarion-dev/clarion`, Go 1.22+
