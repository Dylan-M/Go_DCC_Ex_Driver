package capabilities_test

import (
	"reflect"
	"testing"

	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/capabilities"
)

// IDs and version numbers in these tests are fixtures, not assigned firmware IDs.
func catalog() capabilities.Catalog {
	return capabilities.Catalog{SchemaVersion: 1, Features: []capabilities.Feature{
		{ID: "example.consist", Versions: []uint32{1, 3}, Description: "Test feature", Details: map[string]string{"limit": "8"}},
		{ID: "example.unknown", Versions: []uint32{7}},
	}}
}

func TestCatalogValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value capabilities.Catalog
		valid bool
	}{
		{"empty advertisement", capabilities.Catalog{SchemaVersion: 1}, true},
		{"known and unknown features", catalog(), true},
		{"future schema", capabilities.Catalog{SchemaVersion: 99}, true},
		{"no schema", capabilities.Catalog{}, false},
		{"empty ID", capabilities.Catalog{SchemaVersion: 1, Features: []capabilities.Feature{{Versions: []uint32{1}}}}, false},
		{"padded ID", capabilities.Catalog{SchemaVersion: 1, Features: []capabilities.Feature{{ID: " padded ", Versions: []uint32{1}}}}, false},
		{"duplicate ID", capabilities.Catalog{SchemaVersion: 1, Features: []capabilities.Feature{{ID: "same", Versions: []uint32{1}}, {ID: "same", Versions: []uint32{2}}}}, false},
		{"no versions", capabilities.Catalog{SchemaVersion: 1, Features: []capabilities.Feature{{ID: "feature"}}}, false},
		{"zero version", capabilities.Catalog{SchemaVersion: 1, Features: []capabilities.Feature{{ID: "feature", Versions: []uint32{0}}}}, false},
		{"duplicate version", capabilities.Catalog{SchemaVersion: 1, Features: []capabilities.Feature{{ID: "feature", Versions: []uint32{1, 1}}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.value.Validate(); (err == nil) != tc.valid {
				t.Fatalf("valid=%v, err=%v", tc.valid, err)
			}
		})
	}
}

func TestExactConsumerSupport(t *testing.T) {
	for _, tc := range []struct {
		schema  uint32
		id      string
		version uint32
		want    bool
	}{
		{1, "example.consist", 1, true},
		{1, "example.consist", 3, true},
		{1, "example.consist", 2, false},
		{1, "example.consist", 0, false},
		{0, "example.consist", 1, false},
		{2, "example.consist", 1, false},
		{1, "missing", 1, false},
		{1, "EXAMPLE.CONSIST", 1, false},
		{1, "example.unknown", 1, false},
	} {
		value := catalog()
		if got := value.Supports(tc.schema, tc.id, tc.version); got != tc.want {
			t.Fatalf("%+v: got %v", tc, got)
		}
		for _, status := range []capabilities.Status{capabilities.NotRequested, capabilities.Pending, capabilities.Available, capabilities.Unsupported, capabilities.Inconclusive, capabilities.InvalidResponse} {
			s := capabilities.Snapshot{Status: status, Catalog: value}
			if got := s.Supports(tc.schema, tc.id, tc.version); got != (tc.want && status == capabilities.Available) {
				t.Fatalf("status %v, %+v: got %v", status, tc, got)
			}
		}
	}
}

func TestCatalogClone(t *testing.T) {
	value := catalog()
	copy := value.Clone()
	if !reflect.DeepEqual(copy, value) {
		t.Fatal("clone lost metadata")
	}
	copy.Features[0].ID = "changed"
	copy.Features[0].Versions[0] = 42
	copy.Features[0].Details["limit"] = "changed"
	copy.Features[1].Versions[0] = 99
	if !reflect.DeepEqual(value, catalog()) {
		t.Fatal("clone shares mutable metadata")
	}
	for _, value := range []capabilities.Catalog{{}, {SchemaVersion: 1, Features: []capabilities.Feature{}}} {
		if !reflect.DeepEqual(value, value.Clone()) {
			t.Fatal("clone changed nil/empty representation")
		}
	}
}
