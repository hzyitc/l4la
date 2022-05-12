package l4la

import "errors"

var (
	ErrClose   = errors.New("Closed")
	ErrTimeout = errors.New("Timeout")
)
