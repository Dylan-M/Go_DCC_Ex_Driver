// Package protocol provides the DCC-EX native protocol layer for frame
// extraction, message parsing, and command encoding. It is GUI-independent
// and transport-neutral, depending only on the Go standard library.
package protocol

import (
	"errors"
	"strconv"
	"strings"
)

var (
	ErrSpeedOutOfBounds = errors.New("speed must be 0-126 or -1 for e-stop")
	ErrDirectionInvalid = errors.New("direction must be 0 or 1")
	ErrFuncOutOfBounds  = errors.New("function must be 0-68 (F0-F68)")
	ErrFuncModeBad      = errors.New("function state must be 0 or 1")
	ErrCABInvalid       = errors.New("cab must be 1-10293")
	ErrCVOutOfBounds    = errors.New("CV must be 1-1024")
)

// ValidateLocomotiveState checks a command's locomotive state values are within bounds.
func ValidateLocomotiveState(cab int, speed int, dir int) error {
	if speed < -1 || speed > 127 { // allow up to 127, then cap in ParseLocoState
		return ErrSpeedOutOfBounds
	}
	if dir != 0 && dir != 1 {
		return ErrDirectionInvalid
	}
	if cab < -1 || cab > 10293 {
		return ErrCABInvalid
	}
	return nil
}

// ValidateFunction checks that a function command has valid parameters.
func ValidateFunction(cab int, funcNum int, state int) error {
	if cab < 1 || cab > 10293 {
		return ErrCABInvalid
	}
	if funcNum < 0 || funcNum > 68 {
		return ErrFuncOutOfBounds
	}
	if state != 0 && state != 1 {
		return ErrFuncModeBad
	}
	return nil
}

// ValidatePOMWrite checks parameters for a Program-on-Main write command.
func ValidatePOMWrite(cab int, cv int, value int) error {
	if cab < 1 || cab > 10293 {
		return ErrCABInvalid
	}
	if cv < 1 || cv > 1024 {
		return ErrOutOfBounds
	}
	if value < 0 || value > 255 {
		return ErrOutOfBounds
	}
	return nil
}

// ValidateCommand validates any outbound command and returns an error if invalid.
func ValidateCommand(cmd []byte) error {
	body := trimBrackets(string(cmd))
	if len(body) == 0 {
		return errors.New("empty command")
	}

	parts := splitBody(body)
	opcode := parts[0]

	switch opcode {
	case "t":
		// Format: <t cab [speed dir]>
		if len(parts) < 4 {
			return errors.New("throttle needs cab, speed, dir")
		}
		cab, err := strconv.Atoi(parts[1])
		if err != nil {
			return ErrInvalidFormat
		}
		speed, err := strconv.Atoi(parts[2])
		if err != nil {
			return ErrInvalidFormat
		}
		dir, err := strconv.Atoi(parts[3])
		if err != nil {
			return ErrInvalidFormat
		}
		if err := ValidateLocomotiveState(cab, speed, dir); err != nil {
			return err
		}
	case "F":
		// Format: <F cab func state>
		if len(parts) < 4 {
			return errors.New("function needs cab, func, state")
		}
		cab, err := strconv.Atoi(parts[1])
		if err != nil {
			return ErrInvalidFormat
		}
		funcNum, err := strconv.Atoi(parts[2])
		if err != nil {
			return ErrInvalidFormat
		}
		state, err := strconv.Atoi(parts[3])
		if err != nil {
			return ErrInvalidFormat
		}
		if err := ValidateFunction(cab, funcNum, state); err != nil {
			return err
		}
	case "!":
		// <!> emergency stop - already validated by format check
		return nil
	case "c":
		// <c> - no args needed
		if len(parts) > 1 {
			return errors.New("current query takes no arguments")
		}
		return nil
	case "R", "W", "w":
		// Address/CV operations validated by Parse*Result functions
		return nil
	default:
		// Other opcodes (s, p*, i) don't need validation here
		return nil
	}

	return nil
}

func trimBrackets(s string) string {
	if len(s) < 2 {
		return s
	}
	s = s[1:]        // strip leading <
	s = s[:len(s)-1] // strip trailing >
	return s
}

func splitBody(body string) []string {
	parts := strings.Split(body, " ")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
