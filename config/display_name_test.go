package config

import (
	"strings"
	"testing"
)

func TestDisplayNames(t *testing.T) {
	for _, name := range []string{"", "  ", "Étoile 🚂", strings.Repeat("界", 80)} {
		if err := ValidateDisplayName(name); err != nil {
			t.Fatal(name, err)
		}
	}
	for _, name := range []string{strings.Repeat("界", 81), "\xff", "a\nb", "a\tb", "a\u2028b", "a\u2029b"} {
		if ValidateDisplayName(name) == nil {
			t.Fatalf("accepted %q", name)
		}
		s := DefaultThrottles()
		s.Tabs[0].Name = name
		if s.Validate() == nil {
			t.Fatalf("saved invalid name %q", name)
		}
	}
}
