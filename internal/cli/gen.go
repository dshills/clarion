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
)

// validSections is the set of supported section names for the gen command.
var validSections = map[string]bool{
	"architecture": true,
	"api":          true,
	"data-model":   true,
	"runbook":      true,
}

func newGenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gen <section>",
		Short: "Regenerate a single documentation section",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			section := args[0]

			// Pre-check: clarion-meta.json must exist.
			metaPath := filepath.Join(flagOutput, "clarion-meta.json")
			if _, err := os.Stat(metaPath); err != nil {
				return fmt.Errorf("clarion-meta.json not found in %s. Run clarion pack enterprise first.", flagOutput)
			}

			// 1. Validate section name.
			if !validSections[section] {
				return fmt.Errorf("unsupported section %q. Valid sections: architecture, api, data-model, runbook", section)
			}

			// 2. Load clarion-meta.json.
			fm, err := facts.Load(metaPath)
			if err != nil {
				return fmt.Errorf("load clarion-meta.json: %w", err)
			}

			// 3. Load LLM config.
			cfg, err := llm.LoadConfig()
			if err != nil {
				return fmt.Errorf("LLM config: %w", err)
			}

			// 4. Create adapter + budget + pipeline.
			adapter, err := llm.NewAdapter(cfg)
			if err != nil {
				return fmt.Errorf("LLM adapter: %w", err)
			}
			budget := llm.NewBudgetTracker(cfg.TokenBudget)
			pipeline := llm.NewPipeline(adapter, budget, flagVerbose)

			// 5. Create generator.
			gen, err := generator.New(pipeline)
			if err != nil {
				return fmt.Errorf("create generator: %w", err)
			}

			// 6. Read spec and plan files.
			specBytes, err := os.ReadFile(flagSpec)
			if err != nil {
				return fmt.Errorf("read spec %s: %w", flagSpec, err)
			}
			spec := string(specBytes)

			plan := ""
			if flagPlan != "" {
				if data, err := os.ReadFile(flagPlan); err == nil {
					plan = string(data)
				}
			}

			// 7. Generate the section.
			text, err := gen.GenerateSection(ctx, section, fm, spec, plan)
			if err != nil {
				return fmt.Errorf("generate %s: %w", section, err)
			}

			// 8. Write section.md via renderer.
			r := render.New(flagOutput, flagJSON)
			if err := r.WriteMarkdown(section+".md", text); err != nil {
				return fmt.Errorf("write %s.md: %w", section, err)
			}

			// 9. Extract and write Mermaid diagram.
			mmdFile := sectionMermaidFile[section]
			if err := r.WriteMermaid(mmdFile, text); err != nil {
				return fmt.Errorf("write mermaid for %s: %w", section, err)
			}

			if !flagJSON {
				fmt.Printf("Done. Tokens used: %d\n", budget.Used())
			}
			return nil
		},
	}
}
