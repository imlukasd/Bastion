package money_test

import (
	"errors"
	"math"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/imlukasd/Bastion/internal/platform/money"
)

func mustBPS(t *testing.T, amount, bps int64) int64 {
	t.Helper()
	got, err := money.BPS(amount, bps)
	if err != nil {
		t.Fatalf("BPS(%d, %d): %v", amount, bps, err)
	}
	return got
}

// TD §9.3 and §26 worked examples.
func TestBPS_WorkedExamples(t *testing.T) {
	cases := []struct{ amount, bps, want int64 }{
		{13_118_000, 150, 196_770}, // wallet MDR
		{13_118_000, 250, 327_950}, // card MDR
		{1_400_000, 500, 70_000},   // resale fee
		{1_430_000, 250, 35_750},   // resale card MDR
	}
	for _, c := range cases {
		if got := mustBPS(t, c.amount, c.bps); got != c.want {
			t.Errorf("BPS(%d, %d) = %d, want %d", c.amount, c.bps, got, c.want)
		}
	}
}

// TD §1.1: floor, never rounding up.
func TestBPS_Floors(t *testing.T) {
	cases := []struct{ amount, bps, want int64 }{
		{9_999, 1, 0},          // 0.9999
		{19_999, 5_000, 9_999}, // 9 999.5
		{1, 9_999, 0},          // 0.9999
		{10_000, 1, 1},         // exactly 1
		{10_001, 9_999, 9_999}, // 9 999.9999
	}
	for _, c := range cases {
		if got := mustBPS(t, c.amount, c.bps); got != c.want {
			t.Errorf("BPS(%d, %d) = %d, want %d", c.amount, c.bps, got, c.want)
		}
	}
}

// TD §1.1: amount ≤ 9.2 × 10^14 keeps amount × bps inside int64.
func TestBPS_Bounds(t *testing.T) {
	cases := []struct{ amount, bps, want int64 }{
		{money.MaxBPSAmount, money.MaxBPS, money.MaxBPSAmount},
		{money.MaxBPSAmount, 1, 92_000_000_000},
		{money.MaxBPSAmount, money.MaxBPS - 1, 919_908_000_000_000},
		{0, money.MaxBPS, 0},
		{0, 0, 0},
		{money.MaxBPSAmount, 0, 0},
	}
	for _, c := range cases {
		if got := mustBPS(t, c.amount, c.bps); got != c.want {
			t.Errorf("BPS(%d, %d) = %d, want %d", c.amount, c.bps, got, c.want)
		}
	}
}

// outOfRange lists inputs outside 0 ≤ amount ≤ MaxBPSAmount, 0 ≤ bps ≤ MaxBPS (D1).
func outOfRange() []struct{ amount, bps int64 } {
	return []struct{ amount, bps int64 }{
		{-1, 100},
		{money.MaxBPSAmount + 1, 1},
		{math.MaxInt64, 1},
		{math.MinInt64, 1},
		{1_000, -1},
		{1_000, money.MaxBPS + 1},
		{1_000, math.MaxInt64},
	}
}

func TestBPS_RejectsOutOfRange(t *testing.T) {
	for _, c := range outOfRange() {
		got, err := money.BPS(c.amount, c.bps)
		if !errors.Is(err, money.ErrOutOfRange) {
			t.Errorf("BPS(%d, %d) error = %v, want ErrOutOfRange", c.amount, c.bps, err)
		}
		if got != 0 {
			t.Errorf("BPS(%d, %d) = %d with an error, want 0", c.amount, c.bps, got)
		}
	}
}

func TestSplit_RejectsOutOfRange(t *testing.T) {
	for _, c := range outOfRange() {
		share, rest, err := money.Split(c.amount, c.bps)
		if !errors.Is(err, money.ErrOutOfRange) {
			t.Errorf("Split(%d, %d) error = %v, want ErrOutOfRange", c.amount, c.bps, err)
		}
		if share != 0 || rest != 0 {
			t.Errorf("Split(%d, %d) = %d, %d with an error, want 0, 0", c.amount, c.bps, share, rest)
		}
	}
}

// INV M1: every split is exact; the share is the floor and the remainder goes
// to the other party. Checked with math/big, so no int64 overflow and no
// floating point can hide an error (D17).
func TestSplit_IsExact(t *testing.T) {
	const seed1, seed2 = 20261109, 103 // recorded seed (E33)
	r := rand.New(rand.NewPCG(seed1, seed2))
	tenK := big.NewInt(money.MaxBPS)
	for i := range 100_000 {
		amount, bps := sample(r, i)
		share, rest, err := money.Split(amount, bps)
		if err != nil {
			t.Fatalf("Split(%d, %d): %v", amount, bps, err)
		}
		if share+rest != amount || share < 0 || share > amount {
			t.Fatalf("Split(%d, %d) = %d, %d: not an exact split", amount, bps, share, rest)
		}
		product := new(big.Int).Mul(big.NewInt(amount), big.NewInt(bps))
		low := new(big.Int).Mul(big.NewInt(share), tenK)
		high := new(big.Int).Add(low, tenK)
		if low.Cmp(product) > 0 || product.Cmp(high) >= 0 {
			t.Fatalf("Split(%d, %d) share %d is not floor(amount × bps / 10 000)", amount, bps, share)
		}
	}
}

// sample draws over the whole domain and, every fourth draw, near its edges.
func sample(r *rand.Rand, i int) (amount, bps int64) {
	amount = r.Int64N(money.MaxBPSAmount + 1)
	bps = r.Int64N(money.MaxBPS + 1)
	switch i % 4 {
	case 1:
		amount = money.MaxBPSAmount - r.Int64N(10_000)
	case 2:
		amount = r.Int64N(100_000)
	case 3:
		bps = money.MaxBPS - r.Int64N(10)
	}
	return amount, bps
}
