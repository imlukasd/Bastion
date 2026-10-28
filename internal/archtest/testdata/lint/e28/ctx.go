// Package e28 violates E28: context position and root contexts.
package e28

import "context"

// Do takes the context second.
func Do(n int, ctx context.Context) error { return ctx.Err() }

// Start uses context.Background.
func Start() error { return Do(1, context.Background()) }

// Later uses context.TODO.
func Later() error { return Do(2, context.TODO()) }
