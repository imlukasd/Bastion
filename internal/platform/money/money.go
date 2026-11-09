package money

import (
	"errors"
	"fmt"
)

// Currency is an ISO 4217 code stored as char(3) next to every amount (TD §1.1).
type Currency string

// VND is the Vietnamese dong; exponent 0, so one minor unit is one dong (TD §1.1).
const VND Currency = "VND"

// Bounds of BPS (TD §1.1; bps as in the fee_bps CHECK of TD §19). With both
// bounds, amount × bps ≤ 9.2 × 10^18 < 2^63 − 1, so the product never overflows.
const (
	MaxBPS       int64 = 10_000
	MaxBPSAmount int64 = 920_000_000_000_000 // 9.2 × 10^14
)

// ErrOutOfRange reports an amount or a basis-point rate outside the bounds of BPS.
var ErrOutOfRange = errors.New("money: out of range")

// BPS returns floor(amount × bps / 10 000) (TD §1.1). It returns ErrOutOfRange
// unless 0 ≤ amount ≤ MaxBPSAmount and 0 ≤ bps ≤ MaxBPS.
func BPS(amount, bps int64) (int64, error) {
	if amount < 0 || amount > MaxBPSAmount {
		return 0, fmt.Errorf("%w: amount %d not in [0, %d]", ErrOutOfRange, amount, MaxBPSAmount)
	}
	if bps < 0 || bps > MaxBPS {
		return 0, fmt.Errorf("%w: bps %d not in [0, %d]", ErrOutOfRange, bps, MaxBPS)
	}
	// Both operands are non-negative, so integer division is the floor.
	return amount * bps / MaxBPS, nil
}

// Split returns share = BPS(amount, bps) and rest = amount − share: the floor
// goes to the fee side and the remainder to the other party, so the split is
// exact (INV M1). It returns ErrOutOfRange on the same inputs as BPS.
func Split(amount, bps int64) (share, rest int64, err error) {
	share, err = BPS(amount, bps)
	if err != nil {
		return 0, 0, err
	}
	return share, amount - share, nil
}
