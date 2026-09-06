// Package protocol provides the DCC-EX native protocol layer for frame
// extraction, message parsing, and command encoding. It is GUI-independent
// and transport-neutral, depending only on the Go standard library.
package protocol

import (
	"errors"
	"fmt"
)

// SpeedByte represents a decoded locomotive speed byte from an inbound message.
// The DCC-EX protocol encodes speed and direction in a single byte: bit 7 = direction
// (1=forward, 0=reverse), low 7 bits encode the speed value. Values 0 and 1 in raw
// map to Speed=0 (stop or e-stop); raw 2-127 map to Speed 1-126. This type is used
// for parsing inbound messages; outbound commands use separate Encode* functions.
type SpeedByte struct {
	Speed     int   // 0=stop, 1-126=running; -1=e-stop
	Direction int   // 0=reverse, 1=forward
	Raw       uint8 // original low 7 bits (speed value from packet)
}

// DecodeSpeedByte constructs a SpeedByte from the documented DCC-EX encoding.
// The raw byte has bit 7 set for forward movement; low 7 bits map to speed values
// where 0,1 = stop/e-stop and 2-127 = speeds 1-126. Direction matches the packet's
// bit 7 value; Speed decodes the locomotive's motion state.
func DecodeSpeedByte(raw byte) SpeedByte {
	sb := SpeedByte{}

	// Extract direction from bit 7 of the raw byte
	sb.Direction = int(raw>>7) & 1

	// Extract raw speed from low 7 bits (raw & 0x7F gives 0-127)
	sb.Raw = raw & 0x7F

	// Decode speed value: 0,1 -> stop (0); 2-127 -> speeds 1-126
	if sb.Raw == 0 || sb.Raw == 1 {
		sb.Speed = 0 // stop or e-stop (both map to same output)
	} else {
		sb.Speed = int(sb.Raw) - 1 // raw 2 maps to speed 1, etc.
		if sb.Speed > 126 {
			sb.Speed = 126 // cap at documented ceiling
		}
	}

	return sb
}

// EncodeSpeedByte constructs a speed byte for outbound throttle commands.
// This function reverses DecodeSpeedByte: it takes Speed and Direction and produces
// the raw byte that would be sent on the wire. Speed 0 means stop, speeds 1-126
// are normal running; -1 is e-stop. Direction must be 0 or 1.
func EncodeSpeedByte(speed int, direction int) (byte, error) {
	if speed < -1 || speed > 126 {
		return 0, ErrSpeedOutOfBounds
	}
	if direction != 0 && direction != 1 {
		return 0, ErrDirectionInvalid
	}

	raw := uint8(0)
	sb := SpeedByte{Speed: speed, Direction: direction}

	if speed == -1 {
		// e-stop: raw=0, direction preserved
		raw = 0
	} else if speed > 0 {
		// Running: bit 7 = direction, low 7 bits = speed+1
		sb.Raw = uint8(speed + 1)
		raw = uint8(uint32(sb.Direction)<<7 | uint32(sb.Raw))
	} else {
		// Speed=0 means stop: raw=0, direction preserved
		raw = uint8(uint32(sb.Direction) << 7)
	}

	return byte(raw), nil
}

// EncodeStatus returns the status/version request command.
func EncodeStatus() string { return "<s>" }

// EncodePowerOn constructs a power-on command for MAIN/PROG tracks or ALL.
// An empty variadic argument list (no track specified) means ALL (all tracks).
// Track names: "" = ALL, "MAIN", "PROG" are explicit; other strings like "JOIN"
// are passed through literally to preserve Python behavior. Returns the command
// and nil error for valid inputs.
func EncodePowerOn(track ...string) string {
	const (
		TrackAll  = "" // ALL tracks
		TrackMain = "MAIN"
		TrackProg = "PROG"
	)

	switch len(track) {
	case 0:
		return "<1>" // ALL - no track argument, means all tracks
	case 1:
		t := track[0]
		if t == TrackAll || t == "ALL" {
			return "<1>"
		}
		if t == TrackMain {
			return `<1 MAIN>`
		}
		if t == TrackProg {
			return `<1 PROG>`
		}
		// For JOIN or other tracks, encode literally to preserve Python behavior
		return fmt.Sprintf("<1 %s>", t)
	case 2:
		// Explicit ALL given as varargs like EncodePowerOn("", "") is same as no args
		if track[0] == "" || track[0] == "ALL" {
			return "<1>"
		}
		// Single-element ALL is handled in case 1; here handle explicit ALL list
		if track[0] == "" && track[1] == "" {
			return "<1>"
		}
		return fmt.Sprintf("<1 %s>", track[0])
	default:
		return fmt.Sprintf("<1 %s>", track[0]) // ignore extras, encode first valid arg
	}
}

// EncodePowerOff constructs a power-off command for MAIN/PROG tracks or ALL.
// An empty variadic argument list (no track specified) means ALL (all tracks).
// Track names: "" = ALL, "MAIN", "PROG" are explicit; other strings like "JOIN"
// are passed through literally to preserve Python behavior.
func EncodePowerOff(track ...string) string {
	const (
		TrackAll  = "" // ALL tracks
		TrackMain = "MAIN"
		TrackProg = "PROG"
	)

	switch len(track) {
	case 0:
		return "<0>" // ALL - no track argument, means all tracks
	case 1:
		t := track[0]
		if t == TrackAll || t == "ALL" {
			return "<0>"
		}
		if t == TrackMain {
			return `<0 MAIN>`
		}
		if t == TrackProg {
			return `<0 PROG>`
		}
		// For JOIN or other tracks, encode literally to preserve Python behavior
		return fmt.Sprintf("<0 %s>", t)
	case 2:
		// Explicit ALL given as varargs like EncodePowerOff("", "") is same as no args
		if track[0] == "" || track[0] == "ALL" {
			return "<0>"
		}
		if track[0] == "" && track[1] == "" {
			return "<0>"
		}
		return fmt.Sprintf("<0 %s>", track[0])
	default:
		return fmt.Sprintf("<0 %s>", track[0]) // ignore extras, encode first valid arg
	}
}

// EncodeLocoRequest constructs a locomotive state request for the specified cab.
func EncodeLocoRequest(cab int) (cmd string, err error) {
	if cab < -1 || cab > 10293 {
		return "", ErrCABInvalid
	}
	return fmt.Sprintf("<t %d>", cab), nil
}

// EncodeThrottle constructs a throttle command with speed and direction.
// speed: 0=stop, 1-126; -1=e-stop
// dir: 0=reverse, 1=forward
// Returns ErrSpeedOutOfBounds for speeds outside [-1, 126].
func EncodeThrottle(cab int, speed int, dir int) (cmd string, err error) {
	if cab < -1 || cab > 10293 {
		return "", ErrCABInvalid
	}
	if speed < -1 || speed > 126 {
		return "", ErrSpeedOutOfBounds
	}
	if dir != 0 && dir != 1 {
		return "", ErrDirectionInvalid
	}

	var sb string
	if speed == -1 {
		sb += " -1" // e-stop
	} else if speed > 0 {
		sb += fmt.Sprintf(" %d", speed)
	} else {
		sb += " 0" // stop
	}
	return fmt.Sprintf("<t %d%s %d>", cab, sb, dir), nil
}

// EncodeFunction constructs a function command. func is 0-68 protocol range.
func EncodeFunction(cab int, funcNum int, state int) (cmd string, err error) {
	if cab < -1 || cab > 10293 {
		return "", ErrCABInvalid
	}
	if funcNum < 0 || funcNum > 68 {
		return "", ErrFuncOutOfBounds
	}
	if state != 0 && state != 1 {
		return "", ErrFuncModeBad
	}
	return fmt.Sprintf("<F %d %d %d>", cab, funcNum, state), nil
}

// EncodeEmergencyStop returns the emergency stop-all command.
func EncodeEmergencyStop() string { return "<!>" }

// EncodeCurrentQuery returns the track current query command.
func EncodeCurrentQuery() string { return "<c>" }

// EncodeReadAddress constructs an address read command on the programming track.
func EncodeReadAddress() string { return "<R>" }

// EncodeWriteAddress constructs an address write (cab change) command.
func EncodeWriteAddress(cab int) (cmd string, err error) {
	if cab < -1 || cab > 10293 {
		return "", ErrCABInvalid
	}
	return fmt.Sprintf("<W %d>", cab), nil
}

// EncodeReadCV constructs a CV read command on the programming track.
func EncodeReadCV(cv int) (cmd string, err error) {
	if cv < 1 || cv > 1024 {
		return "", ErrOutOfBounds
	}
	return fmt.Sprintf("<R %d>", cv), nil
}

// EncodeWriteCV constructs a CV write command on the programming track.
func EncodeWriteCV(cv int, value int) (cmd string, err error) {
	if cv < 1 || cv > 1024 {
		return "", ErrOutOfBounds
	}
	if value < 0 || value > 255 {
		return "", ErrOutOfBounds
	}
	return fmt.Sprintf("<W %d %d>", cv, value), nil
}

// EncodeProgramOnMain constructs a POM write command with no reply.
func EncodeProgramOnMain(cab int, cv int, value int) (cmd string, err error) {
	if cab < 1 || cab > 10293 {
		return "", ErrCABInvalid
	}
	if cv < 1 || cv > 1024 {
		return "", ErrOutOfBounds
	}
	if value < 0 || value > 255 {
		return "", ErrOutOfBounds
	}
	return fmt.Sprintf("<w %d %d %d>", cab, cv, value), nil
}

// EncodeEStop constructs an e-stop command for the specified cab.
// This is equivalent to EncodeThrottle with speed=-1.
func EncodeECab(cab int, dir int) (cmd string, err error) {
	if cab < -1 || cab > 10293 {
		return "", ErrCABInvalid
	}
	if dir != 0 && dir != 1 {
		return "", ErrDirectionInvalid
	}
	return EncodeThrottle(cab, -1, dir)
}

// EncodeLocoUpdate constructs a cab update command with new speed/direction.
// Used when selecting a different locomotive address.
func EncodeLocoUpdate(cab int, speed int, dir int) (cmd string, err error) {
	return EncodeThrottle(cab, speed, dir)
}

// EncodeReadCV29 returns a CV29 read command.
func EncodeReadCV29() string { return "<R 29>" }

// EncodeWriteCV29 constructs a CV29 write command.
func EncodeWriteCV29(value int) (cmd string, err error) {
	if value < 0 || value > 255 {
		return "", ErrOutOfBounds
	}
	return fmt.Sprintf("<W 29 %d>", value), nil
}

// ErrInvalidTrackName is returned when a power command receives an unsupported track name.
var ErrInvalidTrackName = errors.New("track must be one of: ALL, MAIN, PROG, or an unknown track name like JOIN")
