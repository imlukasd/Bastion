// Package e05 violates E5: package-level mutable state and init.
package e05

var counter int

func init() { counter = 1 }

// Count returns the counter.
func Count() int { return counter }
