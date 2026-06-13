package broadcast

import "errors"

var (
	// ErrBroadcasterFull is returned when the message channel is full.
	ErrBroadcasterFull = errors.New("broadcaster message channel full")
)
