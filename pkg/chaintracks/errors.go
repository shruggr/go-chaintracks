package chaintracks

import "errors"

var (
	// ErrHeaderNotFound is returned when a header cannot be found
	ErrHeaderNotFound = errors.New("header not found")

	// ErrDuplicateHeader is returned when trying to add a header that already exists
	ErrDuplicateHeader = errors.New("duplicate header")

	// ErrInvalidHeader is returned when a header fails validation
	ErrInvalidHeader = errors.New("invalid header")

	// ErrInsufficientPoW is returned when a header doesn't meet the difficulty target
	ErrInsufficientPoW = errors.New("insufficient proof of work")

	// ErrBrokenChain is returned when a header's previous hash doesn't link to known chain
	ErrBrokenChain = errors.New("broken chain linkage")

	// ErrInvalidTimestamp is returned when a header has an invalid timestamp
	ErrInvalidTimestamp = errors.New("invalid timestamp")
)
