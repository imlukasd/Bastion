// Package e08 violates E8: floating point on a money path.
package e08

import (
	"fmt"
	"strconv"
)

// Fee computes a fee in floating point.
func Fee(amount int64, rate float64) int64 {
	return int64(float64(amount) * rate)
}

// Parse parses an amount as a float.
func Parse(s string) (int64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse amount: %w", err)
	}
	return int64(f), nil
}
