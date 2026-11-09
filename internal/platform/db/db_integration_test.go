package db_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/imlukasd/Bastion/internal/platform/db"
	"github.com/imlukasd/Bastion/internal/platform/db/dbtest"
	"github.com/imlukasd/Bastion/internal/platform/ids"
)

// setup starts a server and opens a pool as pay_owner on bastion_pay with a
// table counters(id, v ≥ 0) holding rows 1 and 2 at 0.
func setup(t *testing.T) *pgxpool.Pool {
	t.Helper()
	t.Parallel()
	pg := dbtest.StartPostgres(t)
	pool, err := db.Open(t.Context(), pg.PayOwnerURL)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(pool.Close)
	exec(t, pool,
		"CREATE TABLE counters (id int PRIMARY KEY, v int NOT NULL DEFAULT 0, CONSTRAINT counters_v_nonneg CHECK (v >= 0))",
		"INSERT INTO counters (id) VALUES (1), (2)",
		"CREATE TABLE attempts (n int NOT NULL)",
	)
	return pool
}

func exec(t *testing.T, pool *pgxpool.Pool, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if _, err := pool.Exec(t.Context(), s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

func queryInt(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func TestOpen_ConnectsAsApp(t *testing.T) {
	t.Parallel()
	pg := dbtest.StartPostgres(t)
	pool, err := db.Open(t.Context(), pg.PayAppURL)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pool.Close()
	var user string
	if err := pool.QueryRow(t.Context(), "SELECT current_user").Scan(&user); err != nil || user != dbtest.PayApp {
		t.Fatalf("current_user = %q, %v; want %s", user, err, dbtest.PayApp)
	}

	const wrong = "wrong-pw-9f3a"
	bad := strings.Replace(pg.PayAppURL, dbtest.PayApp+":"+dbtest.PayApp+"@", dbtest.PayApp+":"+wrong+"@", 1)
	if bad == pg.PayAppURL {
		t.Fatalf("could not replace the password in %s", pg.PayAppURL)
	}
	p, err := db.Open(t.Context(), bad)
	if err == nil {
		p.Close()
		t.Fatal("Open with a wrong password succeeded")
	}
	if strings.Contains(err.Error(), wrong) {
		t.Errorf("Open error %q contains the password (E36)", err)
	}
}

// AD-14: ids.New is stored in a uuid column as a version 7 UUID.
func TestOpen_StoresUUIDv7(t *testing.T) {
	pool := setup(t)
	exec(t, pool, "CREATE TABLE things (id uuid PRIMARY KEY)")
	id := ids.New()
	if _, err := pool.Exec(t.Context(), "INSERT INTO things (id) VALUES ($1)", id); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var back uuid.UUID
	var version int
	if err := pool.QueryRow(t.Context(), "SELECT id, uuid_extract_version(id) FROM things").Scan(&back, &version); err != nil {
		t.Fatalf("select: %v", err)
	}
	if back != id || version != 7 {
		t.Fatalf("stored %s (version %d), want %s (version 7)", back, version, id)
	}
}

// TD §1.5, D9: 40001 and 40P01 are retried up to 3 times (4 attempts); the
// failed attempts leave nothing behind.
func TestInTx_RetriesUpToThreeTimes(t *testing.T) {
	pool := setup(t)
	for code, sentinel := range map[string]error{"40001": db.ErrSerialization, "40P01": db.ErrDeadlock} {
		t.Run(code, func(t *testing.T) {
			calls, err := failTimes(t, pool, code, 3)
			stored := queryInt(t, pool, "SELECT count(*) FROM attempts")
			last := queryInt(t, pool, "SELECT coalesce(max(n), 0) FROM attempts")
			if err != nil || calls != db.MaxRetries+1 || stored != 1 || last != 4 {
				t.Errorf("3 failures: err %v, %d calls, %d rows (last %d); want nil, 4 calls, only attempt 4 stored", err, calls, stored, last)
			}

			calls, err = failTimes(t, pool, code, 4)
			stored = queryInt(t, pool, "SELECT count(*) FROM attempts")
			if !errors.Is(err, sentinel) || calls != db.MaxRetries+1 || stored != 0 {
				t.Errorf("4 failures: err %v, %d calls, %d rows; want %v, 4 calls, nothing stored", err, calls, stored, sentinel)
			}
		})
	}
}

// failTimes runs InTx with a body that records its attempt, then fails with
// code on the first failures attempts.
func failTimes(t *testing.T, pool *pgxpool.Pool, code string, failures int) (int, error) {
	t.Helper()
	exec(t, pool, "TRUNCATE attempts")
	calls := 0
	err := db.InTx(t.Context(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		calls++
		if _, err := tx.Exec(t.Context(), "INSERT INTO attempts (n) VALUES ($1)", calls); err != nil {
			return err
		}
		if calls <= failures {
			return &pgconn.PgError{Code: code}
		}
		return nil
	})
	return calls, err
}

// Race table row 1: a committed concurrent update makes the REPEATABLE READ
// transaction fail with 40001; the whole transaction reruns on a new snapshot.
func TestInTx_RetriesSerializationFailure(t *testing.T) {
	pool := setup(t)
	calls := 0
	err := db.InTx(t.Context(), pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
		calls++
		var v int
		if err := tx.QueryRow(t.Context(), "SELECT v FROM counters WHERE id = 1").Scan(&v); err != nil {
			return err
		}
		if calls == 1 { // another transaction commits after our snapshot
			if _, err := pool.Exec(t.Context(), "UPDATE counters SET v = v + 1 WHERE id = 1"); err != nil {
				t.Errorf("concurrent update: %v", err)
			}
		}
		_, err := tx.Exec(t.Context(), "UPDATE counters SET v = v + 10 WHERE id = 1")
		return err
	})
	if err != nil || calls != 2 {
		t.Fatalf("InTx = %v after %d calls; want nil after 2", err, calls)
	}
	if v := queryInt(t, pool, "SELECT v FROM counters WHERE id = 1"); v != 11 {
		t.Fatalf("v = %d, want 11 (both updates)", v)
	}
}

// Race table row 2: two transactions lock rows 1 and 2 in opposite orders;
// PostgreSQL aborts one with 40P01 and InTx reruns it.
func TestInTx_RetriesDeadlockVictim(t *testing.T) {
	pool := setup(t)
	var calls atomic.Int32
	var barrier sync.WaitGroup // both hold their first lock before either asks for the second
	barrier.Add(2)
	run := func(first, second int) error {
		attempt := 0
		return db.InTx(t.Context(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
			attempt++
			calls.Add(1)
			if _, err := tx.Exec(t.Context(), "UPDATE counters SET v = v + 1 WHERE id = $1", first); err != nil {
				return err
			}
			if attempt == 1 {
				barrier.Done()
				barrier.Wait()
			}
			_, err := tx.Exec(t.Context(), "UPDATE counters SET v = v + 1 WHERE id = $1", second)
			return err
		})
	}
	errs := make(chan error, 2)
	go func() { errs <- run(1, 2) }()
	go func() { errs <- run(2, 1) }()
	for range 2 {
		if err := <-errs; err != nil {
			t.Errorf("InTx: %v", err)
		}
	}
	if n := calls.Load(); n != 3 {
		t.Errorf("%d calls, want 3 (one deadlock victim rerun)", n)
	}
	if n := queryInt(t, pool, "SELECT count(*) FROM counters WHERE id IN (1, 2) AND v = 2"); n != 2 {
		t.Errorf("%d of rows 1, 2 at v = 2, want both (each transaction applied once to each row)", n)
	}
}

// TD §1.5: 23505, 23514, 55P03 and other errors are returned at once, mapped.
func TestInTx_DoesNotRetryOtherErrors(t *testing.T) {
	pool := setup(t)
	boom := errors.New("boom")
	cases := []struct {
		name string
		body func(ctx context.Context, tx pgx.Tx) error
		want error
	}{
		{"23505", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO counters (id) VALUES (1)")
			return err
		}, db.ErrDuplicate},
		{"23514", func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "UPDATE counters SET v = -1 WHERE id = 1")
			return err
		}, db.ErrCheckViolation},
		{"55P03", func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout = '50ms'"); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, "SELECT 1 FROM counters WHERE id = 2 FOR UPDATE")
			return err
		}, db.ErrLockTimeout},
		{"go error", func(context.Context, pgx.Tx) error { return boom }, boom},
	}
	holder, err := pool.Begin(t.Context()) // holds row 2 for the 55P03 case
	if err != nil {
		t.Fatalf("begin holder: %v", err)
	}
	defer func() { _ = holder.Rollback(context.WithoutCancel(t.Context())) }()
	if _, err := holder.Exec(t.Context(), "SELECT 1 FROM counters WHERE id = 2 FOR UPDATE"); err != nil {
		t.Fatalf("lock row 2: %v", err)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			calls := 0
			err := db.InTx(t.Context(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
				calls++
				return c.body(t.Context(), tx)
			})
			if calls != 1 || !errors.Is(err, c.want) {
				t.Fatalf("InTx = %v after %d calls; want %v after 1", err, calls, c.want)
			}
		})
	}
}

// An error raised at COMMIT (a deferred unique constraint) is mapped and not retried.
func TestInTx_MapsCommitErrors(t *testing.T) {
	pool := setup(t)
	exec(t, pool, "CREATE TABLE deferred (k int, CONSTRAINT deferred_k_key UNIQUE (k) DEFERRABLE INITIALLY DEFERRED)")
	calls := 0
	err := db.InTx(t.Context(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		calls++
		_, err := tx.Exec(t.Context(), "INSERT INTO deferred (k) VALUES (1), (1)")
		return err // the violation surfaces only at COMMIT
	})
	var pe *pgconn.PgError
	if calls != 1 || !errors.Is(err, db.ErrDuplicate) || !errors.As(err, &pe) || pe.ConstraintName != "deferred_k_key" {
		t.Fatalf("InTx = %v after %d calls; want ErrDuplicate on deferred_k_key after 1", err, calls)
	}
	if n := queryInt(t, pool, "SELECT count(*) FROM deferred"); n != 0 {
		t.Fatalf("%d rows stored, want 0", n)
	}
}

// D10: the backoff stops when the context ends; the error says both why it
// stopped and what the transaction failed with.
func TestInTx_StopsWhenContextEnds(t *testing.T) {
	pool := setup(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	calls := 0
	err := db.InTx(ctx, pool, pgx.TxOptions{}, func(pgx.Tx) error {
		calls++
		cancel()
		return &pgconn.PgError{Code: "40001"}
	})
	if calls != 1 || !errors.Is(err, context.Canceled) || !errors.Is(err, db.ErrSerialization) {
		t.Fatalf("InTx = %v after %d calls; want context.Canceled and ErrSerialization after 1", err, calls)
	}
}
