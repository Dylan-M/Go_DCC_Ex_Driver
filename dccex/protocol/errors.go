package protocol

import (
	"errors"
)

var (
	// ErrInvalidFormat indicates a message/command doesn't match expected format.
	ErrInvalidFormat = errors.New("invalid message format")
	// ErrOutOfBounds indicates a value is outside its documented range.
	ErrOutOfBounds = errors.New("value out of bounds")
	// ErrCABInvalid indicates cab number is outside 1-10293 or -1 for e-stop.
	ErrCABInvalid = errors.New("cab number invalid")
	// ErrSpeedOutOfBounds indicates speed outside -1..126 range.
	ErrSpeedOutOfBounds = errors.New("speed out of bounds")
	// ErrDirectionInvalid indicates direction is not 0 or 1.
	ErrDirectionInvalid = errors.New("direction must be 0 (reverse) or 1 (forward)")
	// ErrFuncOutOfBounds indicates function number outside 0-68 range.
	ErrFuncOutOfBounds = errors.New("function number out of bounds")
	// ErrFuncModeBad indicates function state is not 0 or 1.
	ErrFuncModeBad = errors.New("function state must be 0 or 1")
	// ErrOverflowError indicates buffer overflow occurred on Write().
	ErrOverflowError = errors.New("buffer overflow: data discarded beyond limit")
	// ErrInvalidTrackName is returned when power command has unsupported track argument.
	ErrInvalidTrackName = errors.New("track name not supported: must be ALL, MAIN, PROG, or literal unknown name like JOIN")
)
