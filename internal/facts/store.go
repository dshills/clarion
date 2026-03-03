package facts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Load reads and deserializes a FactModel from path (typically clarion-meta.json).
// Returns a descriptive error if the file is missing, unreadable, or invalid JSON.
func Load(path string) (*FactModel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var fm FactModel
	if err := json.Unmarshal(data, &fm); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &fm, nil
}

// Save serializes fm to path as pretty-printed JSON.
// Parent directories are created if absent.
func Save(path string, fm *FactModel) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dirs: %w", err)
	}
	data, err := json.MarshalIndent(fm, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal fact model: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Validate checks that fm has a valid schema version and that required fields
// are present. Returns a non-nil error describing the first violation found.
func Validate(fm *FactModel) error {
	if fm == nil {
		return errors.New("fact model is nil")
	}
	if fm.SchemaVersion != SchemaV1 {
		return fmt.Errorf("unsupported schema_version %q (expected %q)", fm.SchemaVersion, SchemaV1)
	}
	if fm.Project.Name == "" {
		return errors.New("project.name is required")
	}
	for i, c := range fm.Components {
		if c.Name == "" {
			return fmt.Errorf("components[%d].name is required", i)
		}
		if len(c.SourceFiles) == 0 {
			return fmt.Errorf("components[%d].source_files is required", i)
		}
	}
	for i, a := range fm.APIs {
		if a.Name == "" {
			return fmt.Errorf("apis[%d].name is required", i)
		}
		if len(a.SourceFiles) == 0 {
			return fmt.Errorf("apis[%d].source_files is required", i)
		}
	}
	return nil
}
