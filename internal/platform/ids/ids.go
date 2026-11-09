package ids

import (
	"errors"
	"fmt"
	"uuid"
)

// ErrInvalid reports a string that is not a canonical UUIDv7.
var ErrInvalid = errors.New("ids: invalid id")

// canonicalLen is the length of the hyphenated form xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
const canonicalLen = 36

// New returns a new UUIDv7 (TD §1.2, AD-14). IDs sort in creation order
// unless the system clock moves backwards.
func New() uuid.UUID {
	return uuid.NewV7()
}

// Parse parses the canonical 36-character form of a UUIDv7 (RFC 9562 variant);
// any other form, version or variant is ErrInvalid.
func Parse(s string) (uuid.UUID, error) {
	if len(s) != canonicalLen {
		return uuid.UUID{}, fmt.Errorf("%w: %d characters, want %d", ErrInvalid, len(s), canonicalLen)
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if v := u[6] >> 4; v != 7 {
		return uuid.UUID{}, fmt.Errorf("%w: version %d, want 7", ErrInvalid, v)
	}
	if variant := u[8] >> 6; variant != 0b10 {
		return uuid.UUID{}, fmt.Errorf("%w: variant %02b, want 10", ErrInvalid, variant)
	}
	return u, nil
}
