package facts

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

const DefaultMaxBytes = 200 * 1024 // 200 KB

// TruncateToSize returns a copy of fm with lowest-confidence entries removed
// until the JSON serialization fits within maxBytes.
//
// Drop order within each collection: ascending ConfidenceScore.
// Collection drop order: APIs → Datastores → Jobs → Integrations → Config.
// Components and Security are never dropped.
//
// Returns the trimmed model, total entries dropped, and an error if the model
// still exceeds maxBytes after all droppable entries are removed.
func TruncateToSize(fm *FactModel, maxBytes int) (*FactModel, int, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}

	out := copyFactModel(fm)

	// Pre-sort each droppable collection by confidence ascending.
	sort.SliceStable(out.APIs, func(i, j int) bool {
		return out.APIs[i].ConfidenceScore < out.APIs[j].ConfidenceScore
	})
	sort.SliceStable(out.Datastores, func(i, j int) bool {
		return out.Datastores[i].ConfidenceScore < out.Datastores[j].ConfidenceScore
	})
	sort.SliceStable(out.Jobs, func(i, j int) bool {
		return out.Jobs[i].ConfidenceScore < out.Jobs[j].ConfidenceScore
	})
	sort.SliceStable(out.Integrations, func(i, j int) bool {
		return out.Integrations[i].ConfidenceScore < out.Integrations[j].ConfidenceScore
	})
	sort.SliceStable(out.Config, func(i, j int) bool {
		return out.Config[i].ConfidenceScore < out.Config[j].ConfidenceScore
	})

	dropped := 0

	// Drain collections one entry at a time (lowest confidence first) until
	// the serialized size is within budget or all droppable entries are gone.
	type dropper func() bool // returns true if an entry was dropped

	droppers := []dropper{
		func() bool {
			if len(out.APIs) == 0 {
				return false
			}
			out.APIs = out.APIs[1:]
			dropped++
			return true
		},
		func() bool {
			if len(out.Datastores) == 0 {
				return false
			}
			out.Datastores = out.Datastores[1:]
			dropped++
			return true
		},
		func() bool {
			if len(out.Jobs) == 0 {
				return false
			}
			out.Jobs = out.Jobs[1:]
			dropped++
			return true
		},
		func() bool {
			if len(out.Integrations) == 0 {
				return false
			}
			out.Integrations = out.Integrations[1:]
			dropped++
			return true
		},
		func() bool {
			if len(out.Config) == 0 {
				return false
			}
			out.Config = out.Config[1:]
			dropped++
			return true
		},
	}

	for _, drop := range droppers {
		for drop() {
			serialized, err := json.Marshal(out)
			if err != nil {
				return nil, dropped, fmt.Errorf("marshal during truncation: %w", err)
			}
			if len(serialized) <= maxBytes {
				return out, dropped, nil
			}
		}
	}

	// Final size check after all droppable entries removed.
	serialized, err := json.Marshal(out)
	if err != nil {
		return nil, dropped, fmt.Errorf("marshal after truncation: %w", err)
	}
	if len(serialized) <= maxBytes {
		return out, dropped, nil
	}

	// Even after dropping everything droppable, still too large.
	fmt.Fprintf(os.Stderr, "WARN: FactModel still %d bytes after dropping all droppable entries.\n", len(serialized))
	type sized struct {
		name  string
		bytes int
	}
	var comps []sized
	for _, c := range out.Components {
		b, _ := json.Marshal(c)
		comps = append(comps, sized{c.Name, len(b)})
	}
	sort.Slice(comps, func(i, j int) bool { return comps[i].bytes > comps[j].bytes })
	for i, c := range comps {
		if i >= 5 {
			break
		}
		fmt.Fprintf(os.Stderr, "  component %q: ~%d bytes\n", c.name, c.bytes)
	}
	return nil, dropped, fmt.Errorf(
		"FactModel too large to send to LLM even after truncation (%d bytes). "+
			"Consider scanning a subdirectory instead of the whole repository, or contact support.",
		len(serialized),
	)
}

// copyFactModel returns a shallow copy of fm with independent slices.
func copyFactModel(fm *FactModel) *FactModel {
	out := *fm
	out.Components = append([]Component{}, fm.Components...)
	out.APIs = append([]APIEndpoint{}, fm.APIs...)
	out.Datastores = append([]Datastore{}, fm.Datastores...)
	out.Jobs = append([]BackgroundJob{}, fm.Jobs...)
	out.Integrations = append([]ExternalIntegration{}, fm.Integrations...)
	out.Config = append([]ConfigVar{}, fm.Config...)
	return &out
}
