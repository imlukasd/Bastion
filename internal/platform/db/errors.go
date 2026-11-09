package db

import "errors"

// Sentinel errors for the SQLSTATEs of TD §1.5.
var (
	ErrDuplicate      = errors.New("db: duplicate")             // 23505
	ErrCheckViolation = errors.New("db: check violation")       // 23514
	ErrSerialization  = errors.New("db: serialization failure") // 40001
	ErrDeadlock       = errors.New("db: deadlock")              // 40P01
	ErrLockTimeout    = errors.New("db: lock timeout")          // 55P03
)

// MapError adds the TD §1.5 sentinel for err's SQLSTATE and keeps err in the chain.
func MapError(err error) error {
	return err // P1-03 G2 stub; step 4 implements it
}
