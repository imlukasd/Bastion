package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open parses url, creates a pgx pool and pings it.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	_, _ = ctx, url
	return nil, errors.New("db: Open not implemented") // P1-03 G2 stub
}
