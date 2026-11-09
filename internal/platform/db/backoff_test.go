package db

import (
	"testing"
	"time"
)

// D10: full jitter, uniform in [0, min(100 ms, 10 ms × 2^(retry−1))).
func TestBackoff_FullJitter(t *testing.T) {
	bounds := map[int]time.Duration{1: 10 * time.Millisecond, 2: 20 * time.Millisecond, 3: 40 * time.Millisecond}
	for retry, bound := range bounds {
		aboveHalf := false
		for range 1_000 {
			d := backoff(retry)
			if d < 0 || d >= bound {
				t.Fatalf("backoff(%d) = %v, want in [0, %v)", retry, d, bound)
			}
			if d > bound/2 {
				aboveHalf = true
			}
		}
		if !aboveHalf {
			t.Errorf("backoff(%d): no sample above %v in 1000; not jittered over the full range", retry, bound/2)
		}
	}
}
