package protocol

import (
	"fmt"
	"strconv"
)

func inRange(field string, n, lo, hi int) error {
	if n < lo || n > hi {
		return fmt.Errorf("%s=%d (want %d..%d): %w", field, n, lo, hi, ErrOutOfRange)
	}
	return nil
}
func number(field, s string, lo, hi int64) (int64, error) {
	digits := s
	if len(digits) > 0 && digits[0] == '-' {
		digits = digits[1:]
	}
	if digits == "" {
		return 0, fmt.Errorf("%s: %w", field, ErrMalformed)
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("%s: %w", field, ErrMalformed)
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < lo || n > hi {
		return 0, fmt.Errorf("%s=%q: %w", field, s, ErrOutOfRange)
	}
	return n, nil
}
