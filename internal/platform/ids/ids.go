package ids

import (
	"errors"
	"uuid"
)

// ErrInvalid reports a string that is not a canonical UUIDv7.
var ErrInvalid = errors.New("ids: invalid id")

// New returns a new UUIDv7 (TD §1.2, AD-14).
func New() uuid.UUID {
	return uuid.UUID{} // P1-03 G2 stub; step 2 implements it
}

// Parse parses the canonical 36-character form of a UUIDv7.
func Parse(s string) (uuid.UUID, error) {
	_ = s
	return uuid.UUID{}, errors.New("ids: not implemented") // P1-03 G2 stub
}
