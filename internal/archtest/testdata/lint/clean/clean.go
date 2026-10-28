// Package clean follows every lint rule and must produce no issue.
package clean

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a status is unknown.
var ErrNotFound = errors.New("not found")

// Status is a closed set.
type Status int

// The statuses.
const (
	Pending Status = iota
	Done
)

// Name names a status.
func Name(s Status) string {
	switch s {
	case Pending:
		return "pending"
	case Done:
		return "done"
	}
	return ""
}

// Fee returns basisPoints of amount in minor units, rounded down.
func Fee(amount, basisPoints int64) int64 { return amount * basisPoints / 10_000 }

// Lookup returns the status named name.
func Lookup(ctx context.Context, name string) (Status, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("lookup %q: %w", name, err)
	}
	for _, s := range []Status{Pending, Done} {
		if Name(s) == name {
			return s, nil
		}
	}
	return 0, fmt.Errorf("lookup %q: %w", name, ErrNotFound)
}

// IsNotFound reports whether err is ErrNotFound.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
