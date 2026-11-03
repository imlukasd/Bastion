// Package migrate applies forward-only SQL migrations (AD-15, E23).
//
// A migration is a file NNNN_slug.sql run by the product's owner role (TD §8.3).
// Each applied file is recorded in public.schema_migrations with its SHA-256,
// so a migration edited after it was applied, or one that disappeared, stops
// the run before anything new is applied. There are no down migrations.
package migrate

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Migration is one forward-only SQL file.
type Migration struct {
	Version  int      // from the NNNN prefix
	Name     string   // file name
	SQL      string   // file contents, run as one simple-protocol query
	Checksum [32]byte // SHA-256 of the file bytes
}

// Errors returned by Load and Up, wrapped with the file or version they concern.
var (
	ErrBadName            = errors.New("migrate: file name is not NNNN_slug.sql")
	ErrDuplicateVersion   = errors.New("migrate: two files share a version")
	ErrEdited             = errors.New("migrate: an applied migration was edited")
	ErrUnknownApplied     = errors.New("migrate: an applied migration has no file")
	ErrOutOfOrder         = errors.New("migrate: a pending migration is older than the latest applied one")
	ErrTransactionControl = errors.New("migrate: a migration ended the runner's transaction (COMMIT, ROLLBACK or END)")
)

// lockKey serializes concurrent runs on one database (pg_advisory_lock).
const lockKey = 7_405_001

const createTable = `CREATE TABLE IF NOT EXISTS public.schema_migrations (
	version    integer PRIMARY KEY,
	name       text NOT NULL,
	checksum   bytea NOT NULL,
	applied_at timestamptz NOT NULL DEFAULT now()
)`

// fileName is NNNN_slug.sql with a lowercase slug.
var fileName = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)

// Load reads every *.sql file at the root of fsys, sorted by version. Other
// files and directories are ignored.
func Load(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: read directory: %w", err)
	}
	var ms []Migration
	names := make(map[int]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		m, err := load(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		if prev, ok := names[m.Version]; ok {
			return nil, fmt.Errorf("%w: %s and %s", ErrDuplicateVersion, prev, m.Name)
		}
		names[m.Version] = m.Name
		ms = append(ms, m)
	}
	slices.SortFunc(ms, func(a, b Migration) int { return cmp.Compare(a.Version, b.Version) })
	return ms, nil
}

// load reads one migration file and parses its version from the name.
func load(fsys fs.FS, name string) (Migration, error) {
	match := fileName.FindStringSubmatch(name)
	if match == nil {
		return Migration{}, fmt.Errorf("%w: %s", ErrBadName, name)
	}
	version, err := strconv.Atoi(match[1])
	if err != nil {
		return Migration{}, fmt.Errorf("%w: %s", ErrBadName, name)
	}
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return Migration{}, fmt.Errorf("migrate: read %s: %w", name, err)
	}
	return Migration{Version: version, Name: name, SQL: string(data), Checksum: sha256.Sum256(data)}, nil
}

// Up applies the pending migrations in version order and returns the versions
// it applied (empty when up to date). Every applied migration is checked
// before anything is applied. Each migration runs in its own transaction with
// its schema_migrations row, so a failing one leaves no trace; Up then returns
// the versions applied before it.
func Up(ctx context.Context, conn *pgx.Conn, ms []Migration) (applied []int, err error) {
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return nil, fmt.Errorf("migrate: lock: %w", err)
	}
	defer func() {
		// Unlock even when ctx is canceled; closing the session would also release it.
		_, uerr := conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", lockKey)
		if uerr != nil && err == nil {
			err = fmt.Errorf("migrate: unlock: %w", uerr)
		}
	}()
	if _, err := conn.Exec(ctx, createTable); err != nil {
		return nil, fmt.Errorf("migrate: create schema_migrations: %w", err)
	}
	pending, err := pendingMigrations(ctx, conn, ms)
	if err != nil {
		return nil, err
	}
	for _, m := range pending {
		if err := apply(ctx, conn, m); err != nil {
			return applied, fmt.Errorf("migrate: %04d: %w", m.Version, err)
		}
		applied = append(applied, m.Version)
	}
	return applied, nil
}

// appliedRow is one row of public.schema_migrations.
type appliedRow struct {
	Version  int32
	Checksum []byte
}

// pendingMigrations checks the applied rows against ms (E23) and returns the
// migrations not applied yet, in version order.
func pendingMigrations(ctx context.Context, conn *pgx.Conn, ms []Migration) ([]Migration, error) {
	rows, err := conn.Query(ctx, "SELECT version, checksum FROM public.schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("migrate: read schema_migrations: %w", err)
	}
	applied, err := pgx.CollectRows(rows, pgx.RowToStructByPos[appliedRow])
	if err != nil {
		return nil, fmt.Errorf("migrate: read schema_migrations: %w", err)
	}
	files := make(map[int]Migration, len(ms))
	for _, m := range ms {
		files[m.Version] = m
	}
	done := make(map[int]bool, len(applied))
	latest := 0
	for _, a := range applied {
		v := int(a.Version)
		m, ok := files[v]
		if !ok {
			return nil, fmt.Errorf("%w: %04d", ErrUnknownApplied, v)
		}
		if !bytes.Equal(a.Checksum, m.Checksum[:]) {
			return nil, fmt.Errorf("%w: %04d", ErrEdited, v)
		}
		done[v] = true
		latest = max(latest, v)
	}
	var pending []Migration
	for _, m := range ms {
		if done[m.Version] {
			continue
		}
		if m.Version < latest {
			return nil, fmt.Errorf("%w: %04d", ErrOutOfOrder, m.Version)
		}
		pending = append(pending, m)
	}
	return pending, nil
}

// apply runs one migration and records it in the same transaction.
func apply(ctx context.Context, conn *pgx.Conn, m Migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback after Commit is a no-op; on any error it undoes the migration.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	// No arguments, so pgx uses the simple protocol and the file may hold several statements.
	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return err
	}
	// A COMMIT, ROLLBACK or END in the file would end this transaction, and the
	// row below would then commit apart from the migration (E23). Whatever the
	// file committed stays; the file must be fixed by hand.
	if status := conn.PgConn().TxStatus(); status != 'T' {
		return ErrTransactionControl
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO public.schema_migrations (version, name, checksum) VALUES ($1, $2, $3)",
		m.Version, m.Name, m.Checksum[:]); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
