package protocol

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Event is a station reply; concrete payloads are exported for type switches.
type Event interface {
	RawFrame() string
	event()
}
type Message struct{ Raw string }

func (m Message) RawFrame() string { return m.Raw }
func (Message) event()             {}

type LocoState struct {
	Message
	Cab, Reg         int
	SpeedByte        byte
	Speed, Direction int // Stop and e-stop both display as speed zero.
	Emergency        bool
	FunctionMask     uint32
}
type PowerState string

const (
	Off      PowerState = "OFF"
	On       PowerState = "ON"
	Overload PowerState = "OVERLOAD"
)

type TrackPower struct {
	Message
	State PowerState
	Track string
}

// HasLimits distinguishes missing limits from explicitly zero values.
type CurrentInfo struct {
	Message
	CurrentMA, MaxMA, TripMA int
	HasLimits                bool
}
type Operation string

const (
	Read  Operation = "read"
	Write Operation = "write"
)

type CVResult struct {
	Message
	Operation Operation
	CV, Value int
}

func (r CVResult) Success() bool { return r.Value != -1 }

type AddressResult struct {
	Message
	Operation Operation
	Address   int
}

func (r AddressResult) Success() bool { return r.Address != -1 }

type VersionInfo struct {
	Message
	Text string
}
type Unknown struct{ Message }

// Parse accepts exactly one bracketed frame. Unknown messages are retained for
// the console. Known message shapes are validated before exposing their values.
func Parse(frame string) (Event, error) {
	if len(frame) > MaxFrameSize {
		return nil, ErrFrameTooLarge
	}
	if len(frame) < 3 || frame[0] != '<' || frame[len(frame)-1] != '>' || !utf8.ValidString(frame) {
		return nil, ErrMalformed
	}
	body := frame[1 : len(frame)-1]
	for _, c := range body {
		if c == '<' || c == '>' || c == 127 || (c < 32 && c != '\t' && c != '\r' && c != '\n') {
			return nil, ErrMalformed
		}
	}
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return nil, ErrMalformed
	}
	m := Message{Raw: frame}
	head, args := fields[0], fields[1:]
	if strings.HasPrefix(head, "i") {
		return VersionInfo{m, strings.TrimSpace(strings.TrimSpace(body)[1:])}, nil
	}
	switch head {
	case "l":
		if len(args) != 4 {
			return nil, ErrMalformed
		}
		bounds := [][2]int64{{1, 10293}, {-1, 2147483647}, {0, 255}, {0, 4294967295}}
		names := []string{"cab", "reg", "speed byte", "function mask"}
		values := [4]int64{}
		for i := range values {
			n, err := number(names[i], args[i], bounds[i][0], bounds[i][1])
			if err != nil {
				return nil, err
			}
			values[i] = n
		}
		raw := byte(values[2])
		low := int(raw & 127)
		speed := 0
		if low > 1 {
			speed = low - 1
		}
		return LocoState{m, int(values[0]), int(values[1]), raw, speed, int(raw >> 7), low == 1, uint32(values[3])}, nil
	case "p0", "p1", "p2":
		if len(args) > 1 {
			return nil, ErrMalformed
		}
		track := "ALL"
		if len(args) == 1 {
			track = args[0]
			for _, c := range track {
				if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
					return nil, ErrMalformed
				}
			}
		}
		state := map[string]PowerState{"p0": Off, "p1": On, "p2": Overload}[head]
		return TrackPower{m, state, track}, nil
	case "c":
		return parseCurrent(m, args)
	case "v", "r", "w":
		if (head == "r" || head == "w") && len(args) == 1 {
			n, err := number("address", args[0], -1, 10293)
			if err != nil {
				return nil, err
			}
			if n == 0 {
				return nil, ErrOutOfRange
			}
			op := Read
			if head == "w" {
				op = Write
			}
			return AddressResult{m, op, int(n)}, nil
		}
		if head == "w" || len(args) != 2 {
			return nil, ErrMalformed
		}
		cv, err := number("cv", args[0], 1, 1024)
		if err != nil {
			return nil, err
		}
		value, err := number("value", args[1], -1, 255)
		if err != nil {
			return nil, err
		}
		op := Read
		if head == "r" {
			op = Write
		}
		return CVResult{m, op, int(cv), int(value)}, nil
	default:
		return Unknown{m}, nil
	}
}

func parseCurrent(m Message, args []string) (Event, error) {
	var values []string
	switch len(args) {
	case 1, 3:
		values = args
	case 8:
		if args[0] != "\"CurrentMAIN\"" || args[2] != "C" || args[3] != "\"Milli\"" || args[4] != "\"0\"" || args[6] != "\"1\"" {
			return nil, ErrMalformed
		}
		values = []string{args[1], args[5], args[7]}
	default:
		return nil, ErrMalformed
	}
	nums := [3]int{}
	for i, s := range values {
		n, err := number("current", s, -2147483648, 2147483647)
		if err != nil {
			return nil, fmt.Errorf("current reply: %w", err)
		}
		nums[i] = int(n)
	}
	return CurrentInfo{m, nums[0], nums[1], nums[2], len(values) == 3}, nil
}
