package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/clarion-dev/clarion/internal/facts"
	"github.com/clarion-dev/clarion/internal/generator"
	"github.com/clarion-dev/clarion/internal/llm"
	"github.com/clarion-dev/clarion/internal/render"
	"github.com/clarion-dev/clarion/internal/scanner"
)

func newPackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack",
		Short: "Pack and generate documentation",
	}
	cmd.AddCommand(newPackEnterpriseCmd())
	return cmd
}

// sectionMermaidFile maps a documentation section name to the mermaid diagram filename.
var sectionMermaidFile = map[string]string{
	"architecture": "component.mmd",
	"api":          "api.mmd",
	"data-model":   "data-model.mmd",
	"runbook":      "runbook.mmd",
}

func newPackEnterpriseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enterprise",
		Short: "Generate all documentation sections and clarion-meta.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// 1. Validate --spec exists and is readable.
			specBytes, err := os.ReadFile(flagSpec)
			if err != nil {
				return fmt.Errorf("read spec %s: %w", flagSpec, err)
			}
			spec := string(specBytes)

			// 2. Load --plan if present (non-fatal if absent or unreadable).
			plan := ""
			if flagPlan != "" {
				if data, err := os.ReadFile(flagPlan); err == nil {
					plan = string(data)
				}
			}

			// 3. Acquire output lock. All subsequent errors must return (not Fatalf)
			// so the deferred unlock executes before process exit.
			unlock, err := AcquireLock(flagOutput)
			if err != nil {
				return err
			}
			defer unlock()

			// 4. Scan the repository (root = directory containing the spec file).
			repoRoot := filepath.Dir(flagSpec)
			s := scanner.New()
			fm, err := s.Scan(repoRoot)
			if err != nil {
				return fmt.Errorf("scan: %w", err)
			}

			// 5. Truncate fact model to LLM token-budget limit if needed.
			fm, _, err = facts.TruncateToSize(fm, facts.DefaultMaxBytes)
			if err != nil {
				return fmt.Errorf("fact model too large: %w", err)
			}

			// 6. Write clarion-meta.json.
			r := render.New(flagOutput, flagJSON)
			if err := r.WriteFactModel(fm); err != nil {
				return fmt.Errorf("write fact model: %w", err)
			}

			// 7. Load LLM config and build the pipeline.
			cfg, err := llm.LoadConfig()
			if err != nil {
				return fmt.Errorf("LLM config: %w", err)
			}
			adapter, err := llm.NewAdapter(cfg)
			if err != nil {
				return fmt.Errorf("LLM adapter: %w", err)
			}
			budget := llm.NewBudgetTracker(cfg.TokenBudget)
			pipeline := llm.NewPipeline(adapter, budget, flagVerbose)

			// 8. Initialise the generator (parses embedded templates).
			gen, err := generator.New(pipeline)
			if err != nil {
				return fmt.Errorf("create generator: %w", err)
			}

			// 9. Generate all four documentation sections.
			sections := []string{"architecture", "api", "data-model", "runbook"}
			for _, section := range sections {
				text, err := gen.GenerateSection(ctx, section, fm, spec, plan)
				if err != nil {
					return fmt.Errorf("generate %s: %w", section, err)
				}

				if err := r.WriteMarkdown(section+".md", text); err != nil {
					return fmt.Errorf("write %s.md: %w", section, err)
				}

				mmdFile := sectionMermaidFile[section]
				if err := r.WriteMermaid(mmdFile, text); err != nil {
					return fmt.Errorf("write mermaid for %s: %w", section, err)
				}
			}

			if !flagJSON {
				fmt.Printf("Done. Tokens used: %d\n", budget.Used())
			}
			return nil
		},
	}
}
