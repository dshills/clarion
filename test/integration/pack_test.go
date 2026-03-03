//go:build integration

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clarion-dev/clarion/internal/facts"
	"github.com/clarion-dev/clarion/internal/generator"
	"github.com/clarion-dev/clarion/internal/llm"
	"github.com/clarion-dev/clarion/internal/scanner"
	"github.com/clarion-dev/clarion/internal/verify"
)

// configuredAdapter builds an LLM adapter from the environment. If CLARION_LLM_PROVIDER
// is not set it defaults to openai; if CLARION_LLM_MODEL is not set it defaults to
// gpt-4o-mini. The test is skipped when the required API key is absent.
func configuredAdapter(t *testing.T) (llm.ProviderAdapter, llm.Config) {
	t.Helper()
	if os.Getenv("CLARION_LLM_PROVIDER") == "" {
		t.Setenv("CLARION_LLM_PROVIDER", "openai")
	}
	if os.Getenv("CLARION_LLM_MODEL") == "" {
		t.Setenv("CLARION_LLM_MODEL", "gpt-4o-mini")
	}
	cfg, err := llm.LoadConfig()
	if err != nil {
		t.Skipf("LLM config not available: %v", err)
	}
	adapter, err := llm.NewAdapter(cfg)
	if err != nil {
		t.Skipf("adapter unavailable for provider %q: %v", cfg.Provider, err)
	}
	return adapter, cfg
}

// TestPackEnterprise runs the full pipeline:
//
//  1. Scan the clarion repo to build a FactModel.
//  2. Generate all four documentation sections via a real LLM call.
//  3. Write the output to a temp directory.
//  4. Run VerifyAll and log the confidence breakdown.
//
// Provider and model are taken from CLARION_LLM_PROVIDER / CLARION_LLM_MODEL
// (defaulting to openai / gpt-4o-mini). The test does not fail on verify
// failures because LLM output is non-deterministic; it fails only on hard
// errors (scan failure, generation failure, I/O error).
func TestPackEnterprise(t *testing.T) {
	root := repoRoot(t)
	adapter, cfg := configuredAdapter(t)

	// ── 1. Scan ──────────────────────────────────────────────────────────────
	s := scanner.New()
	fm, err := s.Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	t.Logf("scan: components=%d apis=%d datastores=%d config=%d",
		len(fm.Components), len(fm.APIs), len(fm.Datastores), len(fm.Config))

	// ── 2. Read spec + plan ──────────────────────────────────────────────────
	specPath := findSpec(root)
	if specPath == "" {
		t.Skip("SPEC.md not found; skipping pack test")
	}
	spec, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read SPEC.md: %v", err)
	}
	plan := ""
	if planPath := findPlan(root); planPath != "" {
		if data, readErr := os.ReadFile(planPath); readErr == nil {
			plan = string(data)
		}
	}

	// ── 3. Generate ──────────────────────────────────────────────────────────
	budget := llm.NewBudgetTracker(cfg.TokenBudget)
	pipeline := llm.NewPipeline(adapter, budget, true)

	gen, err := generator.New(pipeline)
	if err != nil {
		t.Fatalf("generator.New: %v", err)
	}

	ctx := context.Background()
	docs, err := gen.GenerateAll(ctx, fm, string(spec), plan)
	if err != nil {
		t.Fatalf("GenerateAll: %v", err)
	}

	// ── 4. Write output ───────────────────────────────────────────────────────
	outDir := t.TempDir()
	sections := []string{"architecture", "api", "data-model", "runbook"}

	for _, section := range sections {
		text, ok := docs[section]
		if !ok {
			t.Errorf("section %q missing from GenerateAll output", section)
			continue
		}
		if strings.TrimSpace(text) == "" {
			t.Errorf("section %q is empty", section)
			continue
		}
		// Must contain at least one Markdown heading.
		if !strings.Contains(text, "# ") && !strings.Contains(text, "## ") {
			t.Errorf("section %q contains no Markdown headings", section)
		}
		outPath := filepath.Join(outDir, section+".md")
		if err := os.WriteFile(outPath, []byte(text), 0o644); err != nil {
			t.Fatalf("write %s: %v", outPath, err)
		}
		t.Logf("section %s: %d bytes", section, len(text))
	}

	// Save FactModel alongside the docs.
	metaPath := filepath.Join(outDir, "clarion-meta.json")
	if err := facts.Save(metaPath, fm); err != nil {
		t.Fatalf("Save FactModel: %v", err)
	}

	// ── 5. Verify ─────────────────────────────────────────────────────────────
	v := verify.New(outDir)
	reports, allPassed, err := v.VerifyAll(fm)
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}

	for _, rpt := range reports {
		t.Logf("verify[%s]: high=%.1f%% medium=%.1f%% low=%.1f%% failed_claims=%d",
			rpt.Section, rpt.HighConfidence, rpt.MediumConfidence,
			rpt.LowConfidence, len(rpt.FailedClaims))
	}
	if allPassed {
		t.Log("verify: all claims passed")
	} else {
		t.Log("verify: some claims failed (non-deterministic LLM output — not a hard failure)")
	}

	t.Logf("tokens used: %d / %d budget", budget.Used(), cfg.TokenBudget)
}

// TestGenSection regenerates a single section from an existing FactModel.
// This exercises the gen-only path (no re-scan) with a real LLM call.
func TestGenSection(t *testing.T) {
	root := repoRoot(t)
	adapter, cfg := configuredAdapter(t)

	s := scanner.New()
	fm, err := s.Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	specPath := findSpec(root)
	if specPath == "" {
		t.Skip("SPEC.md not found")
	}
	spec, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read SPEC.md: %v", err)
	}

	budget := llm.NewBudgetTracker(cfg.TokenBudget)
	pipeline := llm.NewPipeline(adapter, budget, false)

	gen, err := generator.New(pipeline)
	if err != nil {
		t.Fatalf("generator.New: %v", err)
	}

	ctx := context.Background()
	text, err := gen.GenerateSection(ctx, "architecture", fm, string(spec), "")
	if err != nil {
		t.Fatalf("GenerateSection(architecture): %v", err)
	}
	if strings.TrimSpace(text) == "" {
		t.Error("architecture section is empty")
	}
	t.Logf("architecture: %d bytes, tokens_used=%d", len(text), budget.Used())
}
