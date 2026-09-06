// Package protocol provides the DCC-EX native protocol layer for frame
// extraction, message parsing, and command encoding. It is GUI-independent
// and transport-neutral, depending only on the Go standard library.
package protocol

import (
	"fmt"
	"strconv"
	"strings"
)

// Message represents a parsed inbound DCC-EX native protocol message.
// The opcode identifies the message type; args contains space-separated
// field values. Empty args means no body after the opcode.
type Message struct {
	Opcode string   // opcode without brackets (e.g., "l", "p1", "v", "c", etc.)
	Args   []string // space-separated body fields, or empty slice
	Raw    string   // full message including brackets for debugging
}

// ParseMessage extracts and decodes a single frame. The frame must be in the form
// "<opcode [args...]>". Returns nil if the frame doesn't match the expected format.
func ParseMessage(frame []byte) *Message {
	if len(frame) == 0 {
		return nil
	}

	body := strings.TrimPrefix(string(frame), "<")
	body = strings.TrimSuffix(body, ">")

	parts := strings.Split(body, " ")
	if len(parts) == 0 {
		return &Message{Opcode: "", Args: []string{}, Raw: string(frame)}
	}

	opcode := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}

	return &Message{
		Opcode: opcode,
		Args:   args,
		Raw:    string(frame),
	}
}

// InboundMessage is a typed inbound message interface. It provides type-safe access
// to protocol messages with appropriate validation for each message type.
type InboundMessage interface {
	IsLocoState() bool
	IsTrackPower() bool
	IsCVResult() bool
	IsAddressResult() bool
	IsCurrentInfo() bool
	IsVersionInfo() bool
}

// NewInboundMessage parses a frame and returns a typed InboundMessage. The function
// examines the opcode and returns the appropriate concrete message type with an
// error for malformed frames or unsupported opcodes. Unknown messages are treated
// as errors.
func NewInboundMessage(frame []byte) (InboundMessage, error) {
	msg := ParseMessage(frame)
	if msg == nil {
		return nil, ErrInvalidFormat
	}

	switch msg.Opcode {
	case "l":
		lcst, err := ParseLocoState(msg)
		if err != nil {
			return nil, err
		}
		return &locastateMsg{*lcst}, nil
	case "p0", "p1", "p2":
		tkpwr, err := ParseTrackPower(msg)
		if err != nil {
			return nil, err
		}
		return &trackpowerMsg{*tkpwr}, nil
	case "v", "r":
		cvre, err := ParseCVResult(msg)
		if err != nil {
			return nil, err
		}
		// CV29 is special; treat as success if value is non-negative
		cvre.Success = cvre.Success || (cvre.CV == 29 && cvre.Value >= 0)
		return &cvresultMsg{*cvre}, nil
	case "c":
		cu, err := ParseCurrent(msg)
		if err != nil {
			return nil, err
		}
		return &currentInfoMsg{*cu}, nil
	case "R", "W", "w":
		addr, err := ParseAddressResult(msg)
		if err != nil {
			return nil, err
		}
		return &addressResultMsg{*addr}, nil
	case "s", "i":
		vinfo, _ := ParseVersion(msg)
		return &versionInfoMsg{*vinfo}, nil
	default:
		// Unknown opcode - treat as error but preserve the message for inspection
		return nil, fmt.Errorf("unknown opcode %q", msg.Opcode)
	}
}

// locastateMsg wraps LocoState with type-safe methods.
type locastateMsg struct {
	LocoState
}

func (m *locastateMsg) IsLocoState() bool     { return true }
func (m *locastateMsg) IsTrackPower() bool    { return false }
func (m *locastateMsg) IsCVResult() bool      { return false }
func (m *locastateMsg) IsAddressResult() bool { return false }
func (m *locastateMsg) IsCurrentInfo() bool   { return false }
func (m *locastateMsg) IsVersionInfo() bool   { return false }

// trackpowerMsg wraps TrackPower with type-safe methods.
type trackpowerMsg struct {
	TrackPower
}

func (m *trackpowerMsg) IsLocoState() bool     { return false }
func (m *trackpowerMsg) IsTrackPower() bool    { return true }
func (m *trackpowerMsg) IsCVResult() bool      { return false }
func (m *trackpowerMsg) IsAddressResult() bool { return false }
func (m *trackpowerMsg) IsCurrentInfo() bool   { return false }
func (m *trackpowerMsg) IsVersionInfo() bool   { return false }

// cvresultMsg wraps CVResult with type-safe methods.
type cvresultMsg struct {
	CVResult
}

func (m *cvresultMsg) IsLocoState() bool     { return false }
func (m *cvresultMsg) IsTrackPower() bool    { return false }
func (m *cvresultMsg) IsCVResult() bool      { return true }
func (m *cvresultMsg) IsAddressResult() bool { return false }
func (m *cvresultMsg) IsCurrentInfo() bool   { return false }
func (m *cvresultMsg) IsVersionInfo() bool   { return false }

// currentInfoMsg wraps CurrentInfo with type-safe methods.
type currentInfoMsg struct {
	CurrentInfo
}

func (m *currentInfoMsg) IsLocoState() bool     { return false }
func (m *currentInfoMsg) IsTrackPower() bool    { return false }
func (m *currentInfoMsg) IsCVResult() bool      { return false }
func (m *currentInfoMsg) IsAddressResult() bool { return false }
func (m *currentInfoMsg) IsCurrentInfo() bool   { return true }
func (m *currentInfoMsg) IsVersionInfo() bool   { return false }

// addressResultMsg wraps AddressResult with type-safe methods.
type addressResultMsg struct {
	AddressResult
}

func (m *addressResultMsg) IsLocoState() bool     { return false }
func (m *addressResultMsg) IsTrackPower() bool    { return false }
func (m *addressResultMsg) IsCVResult() bool      { return false }
func (m *addressResultMsg) IsAddressResult() bool { return true }
func (m *addressResultMsg) IsCurrentInfo() bool   { return false }
func (m *addressResultMsg) IsVersionInfo() bool   { return false }

// versionInfoMsg wraps VersionInfo with type-safe methods.
type versionInfoMsg struct {
	VersionInfo
}

func (m *versionInfoMsg) IsLocoState() bool     { return false }
func (m *versionInfoMsg) IsTrackPower() bool    { return false }
func (m *versionInfoMsg) IsCVResult() bool      { return false }
func (m *versionInfoMsg) IsAddressResult() bool { return false }
func (m *versionInfoMsg) IsCurrentInfo() bool   { return false }
func (m *versionInfoMsg) IsVersionInfo() bool   { return true }

// LocoState is a parsed locomotive state message.
// Format: <l cab reg speedByte functMap>
// Cab: locomotive address (1-10293)
// Reg: reminder slot position in the reminder table (-1 means not in reminder table)
type LocoState struct {
	Cab          int    // cab number (1-10293)
	Reg          int    // reminder slot position (-1 = not in reminder table)
	SpeedByte    int    // combined speed+direction byte
	Speed        int    // decoded speed (0=stop, 1-126; -1 = e-stop)
	Direction    int    // 0=reverse, 1=forward
	FunctionMask uint32 // 32-bit mask of active functions
}

// ParseLocoState decodes an "<l cab reg speedByte functMap>" message.
// Cab is the locomotive address (1-10293). Reg is the reminder slot position (-1 = not in table).
// Cab must be 1-10293; -1 belongs to reg, not cab. Returns ErrCABInvalid for out-of-range cab values.
func ParseLocoState(msg *Message) (*LocoState, error) {
	if msg.Opcode != "l" || len(msg.Args) < 4 {
		return nil, ErrInvalidFormat
	}

	cab, err := strconv.Atoi(msg.Args[0])
	if err != nil {
		return nil, ErrInvalidFormat
	}

	// Cab must be in valid range; -1 is for reg (reminder slot), not cab
	if cab < 1 || cab > 10293 {
		return nil, ErrCABInvalid
	}

	// Parse the reminder slot position (reg) from index 1
	regStr := msg.Args[1] // This is the reg field, not cab
	reg, err := strconv.Atoi(regStr)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	// Reg can be -1 (not in reminder table) or a valid slot number
	// Keep reg as-is; application may ignore it if needed

	// Speed byte is at index 2
	speedByte, err := strconv.Atoi(msg.Args[2])
	if err != nil {
		return nil, ErrInvalidFormat
	}

	direction := 1 // default forward (bit 7 = 1 means forward)
	if speedByte&0x80 == 0 {
		direction = 0
	}

	rawSpeed := speedByte & 0x7F
	var speed int
	if rawSpeed == 0 || rawSpeed == 1 {
		// 0 = stop, 1 = e-stop; both map to speed 0 for our purposes
		speed = 0
	} else {
		// raw 2-127 maps to speed 1-126
		speed = rawSpeed - 1
		if speed > 126 {
			speed = 126 // cap at documented ceiling
		}
	}

	funcMap, err := strconv.ParseUint(msg.Args[3], 10, 32)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	return &LocoState{
		Cab:          cab,
		Reg:          reg,
		SpeedByte:    speedByte,
		Speed:        speed,
		Direction:    direction,
		FunctionMask: uint32(funcMap),
	}, nil
}

// ValidateLocoState checks a locomotive state meets the valid range. It returns an error
// if the parsed state has invalid values (speed > 126, cab < -1 or > 10293).
func ValidateLocoState(s *LocoState) error {
	if s.Cab < -1 || s.Cab > 10293 {
		return ErrCABInvalid
	}
	if s.Speed > 126 || s.Speed < -1 {
		return ErrSpeedOutOfBounds
	}
	if s.Direction != 0 && s.Direction != 1 {
		return ErrDirectionInvalid
	}
	if s.FunctionMask > 0xFFFFFFFF {
		return ErrFuncModeBad
	}
	return nil
}

// TrackPower is a parsed power state message.
type TrackPower struct {
	State string // "OFF", "ON", or "OVERLOAD"
	Track string // "ALL", "MAIN", "PROG", or raw value like "JOIN"
}

// ParseTrackPower decodes a "<pN>" or "<pN track>" message where N is 0/1/2.
// p0 = power off, p1 = power on, p2 = overload (latched until cleared).
// Opcode must be exactly "p0", "p1", or "p2". Track argument is optional and
// preserved as-is for known values like "MAIN" or "PROG"; unknown track names
// are passed through to match Python behavior. Returns ErrInvalidFormat for
// invalid opcode length or non-p prefix.
func ParseTrackPower(msg *Message) (*TrackPower, error) {
	if len(msg.Opcode) < 2 || msg.Opcode[0] != 'p' {
		return nil, ErrInvalidFormat
	}

	code := msg.Opcode[1] // '0', '1', or '2'
	switch code {
	case '0':
		// p0: power off, track arg not used
		if len(msg.Args) > 0 {
			return nil, ErrInvalidFormat // p0 should not have args
		}
		return &TrackPower{State: "OFF", Track: ""}, nil
	case '1':
		if len(msg.Args) == 0 {
			return &TrackPower{State: "ON", Track: ""}, nil
		}
		// p1 with track argument - preserve for display (MAIN, PROG, JOIN, etc.)
		return &TrackPower{State: "ON", Track: msg.Args[0]}, nil
	case '2':
		if len(msg.Args) == 0 {
			return &TrackPower{State: "OVERLOAD", Track: ""}, nil
		}
		// p2 with track argument - preserve for display
		return &TrackPower{State: "OVERLOAD", Track: msg.Args[0]}, nil
	default:
		return nil, ErrInvalidFormat // opcode is not p0/p1/p2
	}
}

// ValidateTrackPower checks that the power state is valid. Track names like
// "JOIN" are preserved as-is; we don't validate them against a list.
func ValidateTrackPower(p *TrackPower) error {
	if p.State != "OFF" && p.State != "ON" && p.State != "OVERLOAD" {
		return ErrInvalidFormat
	}
	return nil
}

// CurrentInfo is a parsed current reply message.
type CurrentInfo struct {
	CurrentMA int  // current mA reading
	MaxMA     int  // motor driver capability (may be 0)
	TripMA    int  // circuit breaker trip point
	HasQuoted bool // whether the original included quoted filler fields
}

// ParseCurrent decodes a "<c [name] current C "Milli" "0" max "1" trip>" message.
// We parse by pulling bare integers rather than fixed positions to tolerate
// variations and the shorter form that some builds emit. All reported values are
// preserved exactly as received; we never clamp or alter station state.
func ParseCurrent(msg *Message) (*CurrentInfo, error) {
	if msg.Opcode != "c" {
		return nil, ErrInvalidFormat
	}

	// Extract all quoted strings (filler fields) and bare numbers
	var nums []int
	for _, arg := range msg.Args {
		// Skip quoted strings like "CurrentMAIN", "Milli", "0", "1"
		if strings.HasPrefix(arg, `"`) {
			continue
		}
		// Parse as integer (may be negative for -1)
		n, err := strconv.Atoi(arg)
		if err == nil {
			nums = append(nums, n)
		}
	}

	if len(nums) < 1 {
		return nil, ErrInvalidFormat // bare current without value is invalid
	}

	current := nums[0]
	maxMA := 0
	tripMA := 0

	if len(nums) >= 2 {
		maxMA = nums[1]
	}
	if len(nums) >= 3 {
		tripMA = nums[2]
	}

	// Preserve reported values exactly as received; no clamping per Python impl
	return &CurrentInfo{
		CurrentMA: current,
		MaxMA:     maxMA,
		TripMA:    tripMA,
		HasQuoted: len(msg.Args) > 0 && strings.Contains(msg.Raw, `"`),
	}, nil
}

// CVResult is a parsed CV read/write acknowledgement.
type CVResult struct {
	CV      int    // CV number; may be -1 for failed operations (see Notes below)
	Value   int    // CV value; -1 indicates failure for read results per Python impl
	CVName  string // human-readable name if known
	Success bool   // true for successful operation, false for failure
}

// ParseCVResult decodes both "<v cv value>" (read result) and "<r cv address/value>"
// (write acknowledgement). Disambiguation: a write ack has only one argument,
// a read result has two. CV -1 as the value indicates failure per Python impl;
// CV number is preserved even for failures to match source format exactly.
func ParseCVResult(msg *Message) (*CVResult, error) {
	if len(msg.Args) == 0 {
		return nil, ErrInvalidFormat // <v> or <r> alone is invalid
	}

	cv, err := strconv.Atoi(msg.Args[0])
	if err != nil {
		return nil, ErrInvalidFormat
	}

	// CV1-CV28: two-argument read; CV1-CV1024: one-or-two argument write
	if len(msg.Args) == 2 {
		// Read result: <v cv value> or <r cv value (write ack)>
		val, err := strconv.Atoi(msg.Args[1])
		if err != nil {
			return nil, ErrInvalidFormat
		}

		// CV -1 as value indicates failure for read operations
		success := val >= 0 && val <= 255
		return &CVResult{
			CV:      cv,
			Value:   val,
			Success: success,
		}, nil
	}

	if len(msg.Args) == 1 {
		// Write ack or CV read failure sentinel: <r -1> means failed read
		val, _ := strconv.Atoi(msg.Args[0])
		// For single-arg write acks and read failures, preserve reported value
		success := val >= 0 && val <= 255
		return &CVResult{
			CV:      cv,
			Value:   val,
			Success: success,
		}, nil
	}

	return nil, ErrInvalidFormat
}

// ValidateCVResult checks that the CV value is in a valid range.
func ValidateCVResult(r *CVResult) error {
	if r.CV < 1 || r.CV > 1024 {
		return ErrOutOfBounds
	}
	if r.Value < 0 || r.Value > 255 {
		return ErrOutOfBounds
	}
	return nil
}

// AddressResult is a parsed address read/write acknowledgement.
type AddressResult struct {
	Address int // cab number; -1 indicates failure per Python implementation
	Success bool
}

// ParseAddressResult decodes "<r address>" (read result) and "<w cab>" (write ack).
// Exactly one argument is required. Address -1 indicates a failed read operation
// per Python implementation; the cab number is preserved for display even on failure.
func ParseAddressResult(msg *Message) (*AddressResult, error) {
	if len(msg.Args) != 1 {
		return nil, ErrInvalidFormat
	}

	addrStr := msg.Args[0]
	if addrStr == "" {
		return nil, ErrInvalidFormat // empty argument is invalid
	}

	addr, err := strconv.Atoi(addrStr)
	if err != nil {
		return nil, ErrInvalidFormat
	}

	// Address -1 indicates failure for read; clamp to valid range
	if addr < -1 || addr > 10293 {
		return nil, ErrOutOfBounds
	}

	return &AddressResult{
		Address: addr,
		Success: addr != -1,
	}, nil
}

// ValidateAddressResult checks that the address is valid.
func ValidateAddressResult(r *AddressResult) error {
	if r.Address < -1 || r.Address > 10293 {
		return ErrOutOfBounds
	}
	return nil
}

// CVNames maps common NMRA S-9.2.2 CV numbers to their descriptions.
var CVNames = map[int]string{
	1:   "Primary (short) address",
	2:   "Vstart -- motor start voltage",
	3:   "Acceleration rate",
	4:   "Deceleration rate",
	5:   "Vhigh -- top speed voltage",
	6:   "Vmid -- mid speed voltage",
	7:   "Manufacturer version, read-only",
	8:   "Manufacturer ID -- writing it resets many decoders",
	17:  "Extended address high byte",
	18:  "Extended address low byte",
	19:  "Consist address",
	21:  "Consist functions F1-F8",
	22:  "Consist functions FL, F9-F12",
	23:  "Acceleration adjustment",
	24:  "Deceleration adjustment",
	28:  "RailCom configuration",
	29:  "Configuration data #1",
	30:  "Error information",
	65:  "Kick start",
	66:  "Forward trim",
	95:  "Reverse trim",
	105: "User ID #1",
	106: "User ID #2",
}

// RangeName returns the functional name for CVs in specific ranges.
func RangeName(cv int) string {
	if cv < 1 || cv > 1024 {
		return ""
	}
	if name, ok := CVNames[cv]; ok {
		return fmt.Sprintf("CV %d (%s)", cv, name)
	}
	if 33 <= cv && cv <= 46 {
		return fmt.Sprintf("CV %d (Function output mapping)", cv)
	}
	if 67 <= cv && cv <= 94 {
		n := cv - 66
		return fmt.Sprintf("CV %d (Speed table entry %d/28)", cv, n)
	}
	if 112 <= cv && cv <= 256 {
		return fmt.Sprintf("CV %d (Manufacturer-specific)", cv)
	}
	return fmt.Sprintf("CV %d", cv)
}

// VersionInfo is a parsed version/info message.
type VersionInfo struct {
	Raw string // original message unchanged
}

// ParseVersion decodes an info/version message. We just preserve the raw form for display.
func ParseVersion(msg *Message) (*VersionInfo, error) {
	return &VersionInfo{Raw: msg.Raw}, nil
}
