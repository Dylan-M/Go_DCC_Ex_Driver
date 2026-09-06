package protocol

import "errors"

var (
	// ErrInvalidFormat indicates a message/command doesn't match expected format.
	ErrInvalidFormat = errors.New("invalid message format")
	// ErrOutOfBounds indicates a value is outside its documented range.
	ErrOutOfBounds = errors.New("value out of bounds")
)
