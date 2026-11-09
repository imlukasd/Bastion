package money

import "errors"

// Currency is an ISO 4217 code stored as char(3) next to every amount (TD §1.1).
type Currency string

// VND is the Vietnamese dong; exponent 0, so one minor unit is one dong (TD §1.1).
const VND Currency = "VND"

// Bounds of BPS (TD §1.1; bps as in the fee_bps CHECK of TD §19).
const (
	MaxBPS       int64 = 10_000
	MaxBPSAmount int64 = 920_000_000_000_000 // 9.2 × 10^14
)

// ErrOutOfRange reports an amount or a basis-point rate outside the bounds of BPS.
var ErrOutOfRange = errors.New("money: out of range")

// errNotImplemented marks the P1-03 G2 stubs; step 1 replaces them.
var errNotImplemented = errors.New("money: not implemented")

// BPS returns floor(amount × bps / 10 000) (TD §1.1).
func BPS(amount, bps int64) (int64, error) {
	_, _ = amount, bps
	return 0, errNotImplemented
}

// Split returns share = BPS(amount, bps) and rest = amount − share (INV M1).
func Split(amount, bps int64) (share, rest int64, err error) {
	_, _ = amount, bps
	return 0, 0, errNotImplemented
}
