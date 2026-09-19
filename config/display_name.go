package config

import (
	"errors"
	"unicode"
	"unicode/utf8"
)

// ValidateDisplayName accepts short, single-line Unicode labels. Empty names
// select the UI's default label.
func ValidateDisplayName(name string) error {
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) > 80 {
		return errors.New("names must contain at most 80 valid Unicode characters")
	}
	for _, r := range name {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return errors.New("names must be a single line without control characters")
		}
	}
	return nil
}
