// Package e13 violates E13: unhandled, compared and unwrapped errors.
package e13

import (
	"errors"
	"fmt"
	"os"
)

// ErrX is a sentinel error.
var ErrX = errors.New("x")

// Unchecked ignores an error.
func Unchecked() {
	os.Remove("x")
}

// Compare compares errors with ==.
func Compare(err error) bool { return err == ErrX }

// Wrap wraps with %v.
func Wrap(err error) error { return fmt.Errorf("wrap: %v", err) }
