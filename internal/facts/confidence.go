package facts

// Confidence score tiers used when assigning evidence to FactModel entries.
// Scores are assigned exactly once by the scanner and must not be mutated.
const (
	// ConfidenceDirect indicates a symbol is explicitly registered or declared
	// (e.g. an HTTP handler registered by name on a router).
	ConfidenceDirect = 0.9

	// ConfidenceIndirect indicates a symbol matches a known pattern but has no
	// explicit registration (e.g. function signature matches a handler interface).
	ConfidenceIndirect = 0.7

	// ConfidenceInferred indicates detection via naming convention or partial
	// structural match (e.g. function named FooHandler with no router registration).
	ConfidenceInferred = 0.5

	// ConfidenceSpeculative indicates keyword or comment match only, with no
	// AST-level proof.
	ConfidenceSpeculative = 0.2
)

// IsEvidenceBacked reports whether score meets the evidence-backed threshold
// (>= 0.7). Claims below this threshold must not be presented as verified facts.
func IsEvidenceBacked(score float64) bool { return score >= 0.7 }

// ShouldOmit reports whether score is too low (<0.4) to include in generated
// documentation at all.
func ShouldOmit(score float64) bool { return score < 0.4 }

// NeedsInferredMarker reports whether an output claim must carry the [INFERRED]
// marker — true when the entry is flagged Inferred or its score is in [0.4, 0.7).
func NeedsInferredMarker(score float64, inferred bool) bool {
	return inferred || (score >= 0.4 && score < 0.7)
}
