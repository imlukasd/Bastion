package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxRetries bounds the whole-transaction retries on 40001/40P01 (TD §1.5).
const MaxRetries = 3

// InTx runs fn in one transaction and retries it on 40001/40P01 (TD §1.5, E20).
func InTx(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, fn func(pgx.Tx) error) error {
	_, _, _, _ = ctx, pool, opts, fn
	return errors.New("db: InTx not implemented") // P1-03 G2 stub
}

// backoff returns the full-jitter wait before retry number retry (1-based).
func backoff(retry int) time.Duration {
	_ = retry
	return 0 // P1-03 G2 stub
}
