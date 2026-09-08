package protocol

import "errors"

var (
	ErrMalformed     = errors.New("malformed DCC-EX message")
	ErrOutOfRange    = errors.New("DCC-EX value out of range")
	ErrFrameTooLarge = errors.New("DCC-EX frame exceeds 4096 bytes")
	ErrTruncated     = errors.New("incomplete DCC-EX frame at end of stream")
)
