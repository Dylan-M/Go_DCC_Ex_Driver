// Package capabilities models optional station features independently of their
// wire encoding. It does not assign commands, capability IDs, or feature semantics.
package capabilities

import (
	"fmt"
	"slices"
	"strings"
)

// Feature advertises independently versioned behavior. Details are descriptive
// metadata, not executable code or authority to load a plugin.
type Feature struct {
	ID          string
	Versions    []uint32
	Description string
	Details     map[string]string
}

// Catalog is the decoded discovery response. A future wire adapter must validate
// the response and its framing before passing this model to Discovery.
type Catalog struct {
	SchemaVersion uint32
	Features      []Feature
}

func (c Catalog) Validate() error {
	if c.SchemaVersion == 0 {
		return fmt.Errorf("capability schema version must be positive")
	}
	ids := make(map[string]bool, len(c.Features))
	for _, feature := range c.Features {
		if feature.ID == "" || strings.TrimSpace(feature.ID) != feature.ID || ids[feature.ID] {
			return fmt.Errorf("invalid or duplicate capability ID %q", feature.ID)
		}
		ids[feature.ID] = true
		if len(feature.Versions) == 0 {
			return fmt.Errorf("capability %q must advertise a version", feature.ID)
		}
		versions := make(map[uint32]bool, len(feature.Versions))
		for _, version := range feature.Versions {
			if version == 0 || versions[version] {
				return fmt.Errorf("invalid or duplicate version for capability %q", feature.ID)
			}
			versions[version] = true
		}
	}
	return nil
}

// Supports requires an exact schema and capability version understood by the
// consumer. A newer firmware number or an unknown capability never grants support.
func (c Catalog) Supports(schema uint32, id string, version uint32) bool {
	if schema == 0 || schema != c.SchemaVersion || version == 0 {
		return false
	}
	for _, feature := range c.Features {
		if feature.ID == id && slices.Contains(feature.Versions, version) {
			return true
		}
	}
	return false
}

// Clone detaches all mutable metadata for event-loop and UI snapshots.
func (c Catalog) Clone() Catalog {
	if c.Features == nil {
		return c
	}
	features := make([]Feature, len(c.Features))
	for i, feature := range c.Features {
		features[i] = feature
		features[i].Versions = slices.Clone(feature.Versions)
		if feature.Details != nil {
			features[i].Details = make(map[string]string, len(feature.Details))
			for key, value := range feature.Details {
				features[i].Details[key] = value
			}
		}
	}
	c.Features = features
	return c
}
