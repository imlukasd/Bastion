// Package e21 violates E21: business time from the Go clock.
package e21

import "time"

// Expired decides expiry with the Go clock.
func Expired(deadline time.Time) bool { return time.Now().After(deadline) }

// Age measures age with the Go clock.
func Age(created time.Time) time.Duration { return time.Since(created) }
