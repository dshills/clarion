package generator

import (
	"context"
	"strings"
	"testing"

	"github.com/clarion-dev/clarion/internal/facts"
	"github.com/clarion-dev/clarion/internal/llm"
)

// makeTestPipeline builds a Pipeline backed by a MockAdapter whose fallback
// returns the given text.
func makeTestPipeline(responseText string) *llm.Pipeline {
	adapter := &llm.MockAdapter{
		Responses: map[string]llm.LLMResponse{
			"": {Text: responseText, PromptTokens: 10, CompletionTokens: 20},
		},
	}
	budget := llm.NewBudgetTracker(100000)
	return llm.NewPipeline(adapter, budget, false)
}

// makeFactModel builds a minimal FactModel for testing.
func makeFactModel() *facts.FactModel {
	return &facts.FactModel{
		SchemaVersion: facts.SchemaV1,
		Project:       facts.ProjectInfo{Name: "testproject"},
		Components: []facts.Component{
			{
				Name: "MainServer",
				Evidence: facts.Evidence{
					ConfidenceScore: 0.9,
					Inferred:        false,
					SourceFiles:     []string{"cmd/main.go"},
				},
			},
			{
				Name: "WorkerService",
				Evidence: facts.Evidence{
					ConfidenceScore: 0.5,
					Inferred:        true,
					SourceFiles:     []string{"internal/worker/worker.go"},
				},
			},
			{
				Name: "LegacyModule",
				Evidence: facts.Evidence{
					ConfidenceScore: 0.2, // ShouldOmit
					Inferred:        true,
					SourceFiles:     []string{"internal/legacy/old.go"},
				},
			},
		},
	}
}

// TestGenerateSection_ReturnsLLMText verifies that GenerateSection calls the LLM
// and returns its text with markers applied.
func TestGenerateSection_ReturnsLLMText(t *testing.T) {
	responseText := "WorkerService handles background tasks."
	pipeline := makeTestPipeline(responseText)

	gen, err := New(pipeline)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	fm := makeFactModel()
	result, err := gen.GenerateSection(context.Background(), "architecture", fm, "spec text", "")
	if err != nil {
		t.Fatalf("GenerateSection: %v", err)
	}

	// WorkerService has score=0.5 and inferred=true, so [INFERRED] marker should be appended.
	if !strings.Contains(result, "[INFERRED]") {
		t.Errorf("expected [INFERRED] marker in result, got: %q", result)
	}
}

// TestGenerateSection_MockDeterministic checks that the MockAdapter is called
// and returns a consistent response.
func TestGenerateSection_MockDeterministic(t *testing.T) {
	adapter := &llm.MockAdapter{
		Responses: map[string]llm.LLMResponse{
			"": {Text: "MainServer is the entry point.", PromptTokens: 5, CompletionTokens: 10},
		},
	}
	budget := llm.NewBudgetTracker(100000)
	pipeline := llm.NewPipeline(adapter, budget, false)

	gen, err := New(pipeline)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	fm := makeFactModel()
	result1, err := gen.GenerateSection(context.Background(), "api", fm, "spec", "plan")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Reset budget and adapter call count for second call.
	budget2 := llm.NewBudgetTracker(100000)
	adapter2 := &llm.MockAdapter{
		Responses: map[string]llm.LLMResponse{
			"": {Text: "MainServer is the entry point.", PromptTokens: 5, CompletionTokens: 10},
		},
	}
	pipeline2 := llm.NewPipeline(adapter2, budget2, false)
	gen2, _ := New(pipeline2)
	result2, err := gen2.GenerateSection(context.Background(), "api", fm, "spec", "plan")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if result1 != result2 {
		t.Errorf("expected deterministic results, got different outputs:\n%q\n%q", result1, result2)
	}
}

// TestGenerateSection_UnknownTemplate verifies that an unknown template name returns an error.
func TestGenerateSection_UnknownTemplate(t *testing.T) {
	pipeline := makeTestPipeline("text")
	gen, err := New(pipeline)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	fm := makeFactModel()
	_, err = gen.GenerateSection(context.Background(), "nonexistent-section", fm, "spec", "")
	if err == nil {
		t.Fatal("expected error for unknown template, got nil")
	}
}

// TestApplyInferredMarkers_InferredEntryGetsMarker checks that a line referencing
// an inferred entry gets the [INFERRED] marker appended.
func TestApplyInferredMarkers_InferredEntryGetsMarker(t *testing.T) {
	fm := &facts.FactModel{
		Components: []facts.Component{
			{
				Name: "WorkerService",
				Evidence: facts.Evidence{
					ConfidenceScore: 0.5,
					Inferred:        true,
				},
			},
		},
	}

	text := "WorkerService handles background tasks."
	result := ApplyInferredMarkers(text, fm)

	if !strings.Contains(result, "[INFERRED]") {
		t.Errorf("expected [INFERRED] in result, got: %q", result)
	}
}

// TestApplyInferredMarkers_ShouldOmitLineRemoved checks that a line referencing
// an entry with score < 0.4 is removed entirely.
func TestApplyInferredMarkers_ShouldOmitLineRemoved(t *testing.T) {
	fm := &facts.FactModel{
		Components: []facts.Component{
			{
				Name: "LegacyModule",
				Evidence: facts.Evidence{
					ConfidenceScore: 0.2,
					Inferred:        true,
				},
			},
		},
	}

	text := "LegacyModule is an old subsystem.\nSomething else."
	result := ApplyInferredMarkers(text, fm)

	if strings.Contains(result, "LegacyModule") {
		t.Errorf("expected LegacyModule line to be removed, got: %q", result)
	}
	if !strings.Contains(result, "Something else.") {
		t.Errorf("expected unrelated line to remain, got: %q", result)
	}
}

// TestApplyInferredMarkers_UnrelatedLineUnchanged checks that lines with no
// matching entries are passed through unchanged.
func TestApplyInferredMarkers_UnrelatedLineUnchanged(t *testing.T) {
	fm := &facts.FactModel{
		Components: []facts.Component{
			{
				Name: "MainServer",
				Evidence: facts.Evidence{
					ConfidenceScore: 0.9,
					Inferred:        false,
				},
			},
		},
	}

	text := "This line has nothing to do with the system."
	result := ApplyInferredMarkers(text, fm)

	if result != text {
		t.Errorf("expected unchanged line, got: %q", result)
	}
}

// TestApplyInferredMarkers_NoDoubleAppend checks that [INFERRED] is not
// appended if already present on the line.
func TestApplyInferredMarkers_NoDoubleAppend(t *testing.T) {
	fm := &facts.FactModel{
		Components: []facts.Component{
			{
				Name: "WorkerService",
				Evidence: facts.Evidence{
					ConfidenceScore: 0.5,
					Inferred:        true,
				},
			},
		},
	}

	text := "WorkerService handles tasks. [INFERRED]"
	result := ApplyInferredMarkers(text, fm)

	count := strings.Count(result, "[INFERRED]")
	if count != 1 {
		t.Errorf("expected exactly 1 [INFERRED] marker, got %d in: %q", count, result)
	}
}

// TestApplyInferredMarkers_HighConfidenceNoMarker verifies that high-confidence
// non-inferred entries do not get the [INFERRED] marker.
func TestApplyInferredMarkers_HighConfidenceNoMarker(t *testing.T) {
	fm := &facts.FactModel{
		Components: []facts.Component{
			{
				Name: "MainServer",
				Evidence: facts.Evidence{
					ConfidenceScore: 0.9,
					Inferred:        false,
				},
			},
		},
	}

	text := "MainServer is the primary entrypoint."
	result := ApplyInferredMarkers(text, fm)

	if strings.Contains(result, "[INFERRED]") {
		t.Errorf("expected no [INFERRED] marker for high-confidence entry, got: %q", result)
	}
}
