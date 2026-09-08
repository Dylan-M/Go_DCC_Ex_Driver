package protocol

import "fmt"

func EncodeStatus() string        { return "<s>" }
func EncodeEmergencyStop() string { return "<!>" }
func EncodeCurrentQuery() string  { return "<c>" }
func EncodeReadAddress() string   { return "<R>" }

type Track string

const (
	All  Track = "ALL"
	Main Track = "MAIN"
	Prog Track = "PROG"
)

func EncodePower(on bool, track Track) (string, error) {
	op := 0
	if on {
		op = 1
	}
	switch track {
	case All:
		return fmt.Sprintf("<%d>", op), nil
	case Main, Prog:
		return fmt.Sprintf("<%d %s>", op, track), nil
	default:
		return "", fmt.Errorf("track %q: %w", track, ErrOutOfRange)
	}
}
func EncodeLocoRequest(cab int) (string, error) {
	if err := inRange("cab", cab, 1, 10293); err != nil {
		return "", err
	}
	return fmt.Sprintf("<t %d>", cab), nil
}
func EncodeThrottle(cab, speed, dir int) (string, error) {
	if err := inRange("cab", cab, 1, 10293); err != nil {
		return "", err
	}
	if err := inRange("speed", speed, -1, 126); err != nil {
		return "", err
	}
	if err := inRange("direction", dir, 0, 1); err != nil {
		return "", err
	}
	return fmt.Sprintf("<t %d %d %d>", cab, speed, dir), nil
}
func EncodeFunction(cab, function, state int) (string, error) {
	if err := inRange("cab", cab, 1, 10293); err != nil {
		return "", err
	}
	if err := inRange("function", function, 0, 68); err != nil {
		return "", err
	}
	if err := inRange("state", state, 0, 1); err != nil {
		return "", err
	}
	return fmt.Sprintf("<F %d %d %d>", cab, function, state), nil
}
func EncodeWriteAddress(cab int) (string, error) {
	if err := inRange("cab", cab, 1, 10293); err != nil {
		return "", err
	}
	return fmt.Sprintf("<W %d>", cab), nil
}
func EncodeReadCV(cv int) (string, error) {
	if err := inRange("cv", cv, 1, 1024); err != nil {
		return "", err
	}
	return fmt.Sprintf("<R %d>", cv), nil
}
func EncodeWriteCV(cv, value int) (string, error) {
	if err := inRange("cv", cv, 1, 1024); err != nil {
		return "", err
	}
	if err := inRange("value", value, 0, 255); err != nil {
		return "", err
	}
	return fmt.Sprintf("<W %d %d>", cv, value), nil
}
func EncodeProgramOnMain(cab, cv, value int) (string, error) {
	if err := inRange("cab", cab, 1, 10293); err != nil {
		return "", err
	}
	if err := inRange("cv", cv, 1, 1024); err != nil {
		return "", err
	}
	if err := inRange("value", value, 0, 255); err != nil {
		return "", err
	}
	return fmt.Sprintf("<w %d %d %d>", cab, cv, value), nil
}
