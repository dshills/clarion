package facts

import "time"

// Range represents a span of lines within a source file.
type Range struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Evidence is embedded in every FactModel entry to trace claims back to source.
type Evidence struct {
	SourceFiles     []string `json:"source_files"`
	LineRanges      []Range  `json:"line_ranges"`
	ConfidenceScore float64  `json:"confidence_score"`
	Inferred        bool     `json:"inferred"`
}

// ProjectInfo holds top-level metadata about the scanned repository.
type ProjectInfo struct {
	Name        string   `json:"name"`
	RootPath    string   `json:"root_path"`
	Languages   []string `json:"languages"`
	GoModule    string   `json:"go_module,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Component represents a logical subsystem or package entrypoint.
type Component struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Evidence
}

// APIEndpoint represents a detected HTTP handler registration.
type APIEndpoint struct {
	Name        string `json:"name"`
	Method      string `json:"method"`
	Route       string `json:"route"`
	AuthPattern string `json:"auth_pattern,omitempty"`
	Handler     string `json:"handler,omitempty"`
	Evidence
}

// Datastore represents a detected database or storage dependency.
type Datastore struct {
	Name   string `json:"name"`
	Driver string `json:"driver,omitempty"`
	DSNEnv string `json:"dsn_env,omitempty"`
	Evidence
}

// BackgroundJob represents a detected scheduled or long-running goroutine.
type BackgroundJob struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule,omitempty"`
	Evidence
}

// ExternalIntegration represents a detected outbound HTTP dependency.
type ExternalIntegration struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url,omitempty"`
	Evidence
}

// ConfigVar represents a detected environment variable or configuration key.
type ConfigVar struct {
	Name         string `json:"name"`
	EnvKey       string `json:"env_key"`
	DefaultValue string `json:"default_value,omitempty"`
	Required     bool   `json:"required"`
	Evidence
}

// SecurityModel captures detected auth mechanisms and trust boundaries.
type SecurityModel struct {
	AuthMechanism   string   `json:"auth_mechanism,omitempty"`
	TrustBoundaries []string `json:"trust_boundaries,omitempty"`
	Evidence
}

// FactModel is the canonical internal data structure produced by the scanner
// and consumed by the documentation generator and verification engine.
// It is serialized to clarion-meta.json as the authoritative evidence store.
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

// SchemaV1 is the schema_version value for all v1 output.
const SchemaV1 = "1.0"
