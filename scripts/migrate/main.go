// Command migrate applies one product's forward-only migrations (P1-02,
// internal/platform/db/migrate) as that product's owner role (TD §8.3). It is
// the runner behind `task migrate`.
//
//	go run ./scripts/migrate -url <owner URL> -dir db/pay/migrations
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/imlukasd/Bastion/internal/platform/db/migrate"
)

func main() {
	url := flag.String("url", "", "PostgreSQL URL of the product's owner role")
	dir := flag.String("dir", "", "directory of NNNN_slug.sql migrations")
	flag.Parse()
	if *url == "" || *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: migrate -url <owner URL> -dir <migrations dir>")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Stdout, *url, *dir)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run applies the migrations in dir and reports what it applied.
func run(ctx context.Context, w io.Writer, url, dir string) error {
	ms, err := migrate.Load(os.DirFS(dir))
	if err != nil {
		return fmt.Errorf("migrate %s: %w", dir, err)
	}
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("migrate %s: connect: %w", dir, err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	applied, err := migrate.Up(ctx, conn, ms)
	if err != nil {
		return fmt.Errorf("migrate %s: %w", dir, err)
	}
	if len(applied) == 0 {
		_, err = fmt.Fprintf(w, "migrate %s: up to date\n", dir)
		return err
	}
	versions := make([]string, len(applied))
	for i, v := range applied {
		versions[i] = fmt.Sprintf("%04d", v)
	}
	_, err = fmt.Fprintf(w, "migrate %s: applied %s\n", dir, strings.Join(versions, " "))
	return err
}
