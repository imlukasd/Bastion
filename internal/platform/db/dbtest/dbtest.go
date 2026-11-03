// Package dbtest starts real PostgreSQL and Redis servers for integration
// tests (E29, TD §34.1), built like the local stack in deploy/compose.yaml:
// the same images, track_commit_timestamp = on, the same init script with the
// databases and roles, and the products' migrations.
//
// Only tests import this package. Every Start function skips under -short and
// fails the test when the server cannot start (Docker missing included).
package dbtest

import (
	"context"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/imlukasd/Bastion/internal/platform/db/migrate"
)

// Images of the local stack; TestStack_ImagesMatchCompose keeps them equal to deploy/compose.yaml.
const (
	PostgresImage = "postgres:18.6-alpine3.24"
	RedisImage    = "redis:8.10.3-alpine"
)

// TrackCommitTimestamp is the server flag both the stack and the tests pass (TD §1.3).
const TrackCommitTimestamp = "track_commit_timestamp=on"

// Databases and roles created by deploy/postgres/init/10-bastion.sql (TD §1.4, §8.3).
const (
	Superuser       = "postgres"
	PayDatabase     = "bastion_pay"
	TicketsDatabase = "bastion_tickets"
	PayOwner        = "pay_owner"
	PayApp          = "pay_app"
	TicketsOwner    = "tickets_owner"
	TicketsApp      = "tickets_app"
)

// Postgres is a running test server with both databases and their roles.
type Postgres struct {
	SuperuserURL    string // postgres, database postgres
	PayOwnerURL     string // pay_owner → bastion_pay
	PayAppURL       string // pay_app → bastion_pay
	TicketsOwnerURL string // tickets_owner → bastion_tickets
	TicketsAppURL   string // tickets_app → bastion_tickets

	hostPort string
}

// URL returns a URL for role on database. Each role's password is its name
// (local only, P1-02 D7).
func (p *Postgres) URL(role, database string) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(role, role),
		Host:     p.hostPort,
		Path:     "/" + database,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

// StartPostgres starts PostgresImage with track_commit_timestamp = on and
// deploy/postgres/init/10-bastion.sql, then applies the db/pay and db/tickets
// migrations as the owner roles.
func StartPostgres(t testing.TB) *Postgres {
	t.Helper()
	p := StartPostgresUnmigrated(t)
	root := ModuleRoot(t)
	migrateDir(t, p.PayOwnerURL, filepath.Join(root, "db", "pay", "migrations"))
	migrateDir(t, p.TicketsOwnerURL, filepath.Join(root, "db", "tickets", "migrations"))
	return p
}

// StartPostgresUnmigrated is StartPostgres without the migrations.
func StartPostgresUnmigrated(t testing.TB) *Postgres {
	t.Helper()
	skipShort(t)
	ctx := t.Context()
	initScript := filepath.Join(ModuleRoot(t), "deploy", "postgres", "init", "10-bastion.sql")
	ctr, err := postgres.Run(ctx, PostgresImage,
		postgres.WithInitScripts(initScript),
		postgres.WithDatabase(Superuser),
		postgres.WithUsername(Superuser),
		postgres.WithPassword(Superuser),
		postgres.BasicWaitStrategies(),
		testcontainers.WithCmd("postgres", "-c", TrackCommitTimestamp),
	)
	terminateOnCleanup(t, ctr)
	if err != nil {
		t.Fatalf("dbtest: start PostgreSQL: %v", err)
	}
	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("dbtest: PostgreSQL host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("dbtest: PostgreSQL port: %v", err)
	}
	p := &Postgres{hostPort: net.JoinHostPort(host, port.Port())}
	p.SuperuserURL = p.URL(Superuser, Superuser)
	p.PayOwnerURL = p.URL(PayOwner, PayDatabase)
	p.PayAppURL = p.URL(PayApp, PayDatabase)
	p.TicketsOwnerURL = p.URL(TicketsOwner, TicketsDatabase)
	p.TicketsAppURL = p.URL(TicketsApp, TicketsDatabase)
	return p
}

// Connect opens a connection to url, closed when the test ends.
func Connect(t testing.TB, url string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), url)
	if err != nil {
		t.Fatalf("dbtest: connect: %v", err)
	}
	cleanupCtx := context.WithoutCancel(t.Context())
	t.Cleanup(func() { _ = conn.Close(cleanupCtx) })
	return conn
}

// ModuleRoot returns the repository root: the directory holding go.mod,
// found by walking up from this source file.
func ModuleRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("dbtest: cannot locate the module root")
	}
	for dir := filepath.Dir(file); ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("dbtest: no go.mod above " + file)
		}
		dir = parent
	}
}

// migrateDir applies the migrations in dir through url.
func migrateDir(t testing.TB, url, dir string) {
	t.Helper()
	ms, err := migrate.Load(os.DirFS(dir))
	if err != nil {
		t.Fatalf("dbtest: load %s: %v", dir, err)
	}
	if _, err := migrate.Up(t.Context(), Connect(t, url), ms); err != nil {
		t.Fatalf("dbtest: migrate %s: %v", dir, err)
	}
}

// skipShort skips integration tests under -short, so task check needs no Docker.
func skipShort(t testing.TB) {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: needs Docker (skipped with -short)")
	}
}

// terminateOnCleanup stops the container when the test ends. t.Context() is
// canceled before cleanups run, so the cleanup keeps its values but not its cancellation.
func terminateOnCleanup(t testing.TB, ctr testcontainers.Container) {
	t.Helper()
	cleanupCtx := context.WithoutCancel(t.Context())
	t.Cleanup(func() {
		if ctr == nil {
			return
		}
		if err := ctr.Terminate(cleanupCtx); err != nil {
			t.Errorf("dbtest: terminate container: %v", err)
		}
	})
}
