package db

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxRetries bounds the whole-transaction retries on 40001/40P01 (TD §1.5):
// at most MaxRetries+1 attempts.
const MaxRetries = 3

// Full-jitter backoff bounds (P1-03 D10).
const (
	baseBackoff = 10 * time.Millisecond
	maxBackoff  = 100 * time.Millisecond
)

// InTx runs fn in one transaction with opts: it commits when fn returns nil
// and rolls back when fn returns an error or panics. When fn or the commit
// fails with 40001 or 40P01, it waits a jittered backoff and reruns the whole
// transaction, at most MaxRetries times (TD §1.5). It returns MapError of the
// last error; if ctx ends during a backoff, ctx's error joined with it.
//
// fn may run several times, so it must have no effect outside the
// transaction (E20). InTx is not reentrant: fn uses tx, never pool.
func InTx(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, fn func(pgx.Tx) error) error {
	for retry := 0; ; retry++ {
		err := pgx.BeginTxFunc(ctx, pool, opts, fn)
		if err == nil {
			return nil
		}
		if !retryable(err) || retry == MaxRetries {
			return MapError(err)
		}
		if waitErr := sleep(ctx, backoff(retry+1)); waitErr != nil {
			return errors.Join(waitErr, MapError(err))
		}
	}
}

// retryable reports a serialization failure or a deadlock anywhere in err's chain.
func retryable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == codeSerialization || pgErr.Code == codeDeadlock)
}

// sleep waits d or until ctx ends. ctx is checked first: d may be 0, and a
// select with both channels ready picks one at random.
func sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// backoff returns the full-jitter wait before retry number retry (1-based):
// uniform in [0, min(maxBackoff, baseBackoff × 2^(retry−1))) (E25). Go's clock
// is fine here: backoff is not business time (E21).
func backoff(retry int) time.Duration {
	bound := min(maxBackoff, baseBackoff<<(retry-1))
	return time.Duration(rand.Int64N(int64(bound)))
}
