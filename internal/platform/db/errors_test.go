package db_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/imlukasd/Bastion/internal/platform/db"
)

func sentinels() []error {
	return []error{db.ErrDuplicate, db.ErrCheckViolation, db.ErrSerialization, db.ErrDeadlock, db.ErrLockTimeout}
}

// TD §1.5: each SQLSTATE maps to exactly one sentinel; the PostgreSQL error,
// with its constraint name, stays in the chain (D12).
func TestMapError(t *testing.T) {
	cases := []struct {
		code string
		want error
	}{
		{"23505", db.ErrDuplicate},
		{"23514", db.ErrCheckViolation},
		{"40001", db.ErrSerialization},
		{"40P01", db.ErrDeadlock},
		{"55P03", db.ErrLockTimeout},
	}
	for _, c := range cases {
		pgErr := &pgconn.PgError{Code: c.code, ConstraintName: "c_" + c.code}
		checkMapped(t, c.code+" bare", db.MapError(pgErr), c.want, "c_"+c.code)
		checkMapped(t, c.code+" wrapped", db.MapError(fmt.Errorf("insert: %w", pgErr)), c.want, "c_"+c.code)
	}
}

// checkMapped asserts got matches only want among the sentinels, keeps the
// PgError with its constraint, and is unchanged by a second MapError.
func checkMapped(t *testing.T, name string, got, want error, constraint string) {
	t.Helper()
	for _, s := range sentinels() {
		if is, wantIs := errors.Is(got, s), errors.Is(want, s); is != wantIs {
			t.Errorf("%s: errors.Is(%v, %v) = %v, want %v", name, got, s, is, wantIs)
		}
	}
	var pe *pgconn.PgError
	if !errors.As(got, &pe) || pe.ConstraintName != constraint {
		t.Errorf("%s: PgError with constraint %s lost: %v", name, constraint, got)
	}
	if twice := db.MapError(got); twice.Error() != got.Error() || !errors.Is(twice, want) {
		t.Errorf("%s: mapping twice %q differs from once %q", name, twice, got)
	}
}

func TestMapError_PassesOthersThrough(t *testing.T) {
	if db.MapError(nil) != nil {
		t.Error("MapError(nil) != nil")
	}
	fk := &pgconn.PgError{Code: "23503"}
	plain := errors.New("boom")
	for _, in := range []error{fk, plain} {
		got := db.MapError(in)
		if got != in { //nolint:errorlint // D12: unknown errors are returned unchanged, the same value
			t.Errorf("MapError(%v) = %v, want the same error", in, got)
		}
	}
}
