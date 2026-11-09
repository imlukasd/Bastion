package db

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Sentinel errors for the SQLSTATEs of TD §1.5. SQLSTATEs are mapped here
// and nowhere else (E14); modules turn these into their own sentinels.
var (
	ErrDuplicate      = errors.New("db: duplicate")             // 23505: already done, never retried
	ErrCheckViolation = errors.New("db: check violation")       // 23514: a guard or bound was hit
	ErrSerialization  = errors.New("db: serialization failure") // 40001: retried by InTx
	ErrDeadlock       = errors.New("db: deadlock")              // 40P01: retried by InTx
	ErrLockTimeout    = errors.New("db: lock timeout")          // 55P03: API 503, worker next tick
)

// SQLSTATEs that InTx retries (TD §1.5).
const (
	codeSerialization = "40001"
	codeDeadlock      = "40P01"
)

// sentinelFor returns the sentinel for a SQLSTATE, or nil.
func sentinelFor(code string) error {
	switch code {
	case "23505":
		return ErrDuplicate
	case "23514":
		return ErrCheckViolation
	case codeSerialization:
		return ErrSerialization
	case codeDeadlock:
		return ErrDeadlock
	case "55P03":
		return ErrLockTimeout
	}
	return nil
}

// MapError adds the TD §1.5 sentinel for err's SQLSTATE and keeps err in the
// chain, so errors.Is finds the sentinel and errors.As the *pgconn.PgError
// with its constraint name. nil, errors without a PostgreSQL error, unmapped
// SQLSTATEs and errors already mapped are returned unchanged.
func MapError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	sentinel := sentinelFor(pgErr.Code)
	if sentinel == nil || errors.Is(err, sentinel) {
		return err
	}
	return fmt.Errorf("%w: %w", sentinel, err)
}
