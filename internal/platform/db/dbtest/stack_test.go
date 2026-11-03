package dbtest_test

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"

	"github.com/imlukasd/Bastion/internal/platform/db/dbtest"
)

// product is one product's database with its owner and app roles.
type product struct {
	database, owner, app string
	migrations           string // migrations directory, relative to the module root
}

func products() []product {
	return []product{
		{dbtest.PayDatabase, dbtest.PayOwner, dbtest.PayApp, "db/pay/migrations"},
		{dbtest.TicketsDatabase, dbtest.TicketsOwner, dbtest.TicketsApp, "db/tickets/migrations"},
	}
}

// Compose and the tests must run the same stores (P1-02 D9).
func TestStack_ImagesMatchCompose(t *testing.T) {
	compose, err := os.ReadFile(filepath.Join(dbtest.ModuleRoot(t), "deploy", "compose.yaml"))
	if err != nil {
		t.Fatalf("read compose.yaml: %v", err)
	}
	for _, want := range []string{
		"image: " + dbtest.PostgresImage,
		"image: " + dbtest.RedisImage,
		`"-c", "` + dbtest.TrackCommitTimestamp + `"`,
	} {
		if !strings.Contains(string(compose), want) {
			t.Errorf("deploy/compose.yaml does not contain %q", want)
		}
	}
}

// Clause 1: PostgreSQL >= 17, both databases, track_commit_timestamp = on (TD §1.3, §1.4).
func TestStack_PostgresSettings(t *testing.T) {
	t.Parallel()
	pg := dbtest.StartPostgres(t)
	for _, p := range products() {
		conn := dbtest.Connect(t, pg.URL(p.app, p.database))
		var version int
		var track, database string
		if err := conn.QueryRow(t.Context(), `SELECT current_setting('server_version_num')::int,
			current_setting('track_commit_timestamp'), current_database()`).Scan(&version, &track, &database); err != nil {
			t.Fatalf("%s: settings: %v", p.database, err)
		}
		if version < 170000 {
			t.Errorf("%s: server_version_num = %d, want >= 170000", p.database, version)
		}
		if track != "on" {
			t.Errorf("%s: track_commit_timestamp = %s, want on", p.database, track)
		}
		if database != p.database {
			t.Errorf("current_database() = %s, want %s", database, p.database)
		}
	}
}

// Clause 1: Redis >= 7.2.
func TestStack_RedisVersion(t *testing.T) {
	t.Parallel()
	client := redisClient(t, dbtest.StartRedis(t))
	info, err := client.Info(t.Context(), "server").Result()
	if err != nil {
		t.Fatalf("INFO server: %v", err)
	}
	version := infoField(info, "redis_version")
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		t.Fatalf("redis_version = %q, want major.minor.patch", version)
	}
	major, errMajor := strconv.Atoi(parts[0])
	minor, errMinor := strconv.Atoi(parts[1])
	if errMajor != nil || errMinor != nil {
		t.Fatalf("redis_version = %q, want numbers", version)
	}
	if major < 7 || (major == 7 && minor < 2) {
		t.Errorf("redis_version = %s, want >= 7.2", version)
	}
}

// Clause 2 and D8: the baseline is recorded with its checksum, app roles have
// no access to schema public, and owner functions are not executable by PUBLIC.
func TestStack_MigrationsApplied(t *testing.T) {
	t.Parallel()
	pg := dbtest.StartPostgres(t)
	for _, p := range products() {
		conn := dbtest.Connect(t, pg.URL(p.owner, p.database))
		checkBaselineRecorded(t, conn, p)
		checkBaselinePrivileges(t, conn, p)
	}
}

// checkBaselineRecorded asserts schema_migrations holds exactly version 1 with
// the SHA-256 of the baseline file.
func checkBaselineRecorded(t *testing.T, conn *pgx.Conn, p product) {
	t.Helper()
	file, err := os.ReadFile(filepath.Join(dbtest.ModuleRoot(t), p.migrations, "0001_baseline.sql"))
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	rows, err := conn.Query(t.Context(), "SELECT version, checksum FROM public.schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("%s: schema_migrations: %v", p.database, err)
	}
	type row struct {
		Version  int32
		Checksum []byte
	}
	got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
	if err != nil {
		t.Fatalf("%s: schema_migrations: %v", p.database, err)
	}
	want := sha256.Sum256(file)
	if len(got) != 1 || got[0].Version != 1 || !slices.Equal(got[0].Checksum, want[:]) {
		t.Errorf("%s: schema_migrations = %+v, want one row (1, %x)", p.database, got, want)
	}
}

// checkBaselinePrivileges asserts D8: no USAGE on schema public for the app
// role, and no EXECUTE on a new owner function without a grant.
func checkBaselinePrivileges(t *testing.T, conn *pgx.Conn, p product) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), "CREATE FUNCTION public.probe() RETURNS int LANGUAGE sql AS 'SELECT 1'"); err != nil {
		t.Fatalf("%s: create function: %v", p.database, err)
	}
	var publicUsage, execute bool
	if err := conn.QueryRow(t.Context(),
		"SELECT has_schema_privilege($1, 'public', 'USAGE'), has_function_privilege($1, 'public.probe()', 'EXECUTE')",
		p.app).Scan(&publicUsage, &execute); err != nil {
		t.Fatalf("%s: privileges: %v", p.database, err)
	}
	if publicUsage {
		t.Errorf("%s: %s has USAGE on schema public, want none", p.database, p.app)
	}
	if execute {
		t.Errorf("%s: %s may execute an owner function without a grant", p.database, p.app)
	}
}

// Clause 3 and D6: the app roles exist without elevated attributes and reach
// only their own product's database.
func TestStack_Roles(t *testing.T) {
	t.Parallel()
	pg := dbtest.StartPostgres(t)
	super := dbtest.Connect(t, pg.SuperuserURL)
	for _, role := range []string{dbtest.PayOwner, dbtest.PayApp, dbtest.TicketsOwner, dbtest.TicketsApp} {
		checkRoleAttributes(t, super, role)
	}
	for _, p := range products() {
		var user string
		err := dbtest.Connect(t, pg.URL(p.app, p.database)).QueryRow(t.Context(), "SELECT current_user").Scan(&user)
		if err != nil || user != p.app {
			t.Errorf("%s on %s: current_user = %q (%v)", p.app, p.database, user, err)
		}
	}
	for _, c := range []struct{ role, database string }{
		{dbtest.PayApp, dbtest.TicketsDatabase},
		{dbtest.PayOwner, dbtest.TicketsDatabase},
		{dbtest.TicketsApp, dbtest.PayDatabase},
		{dbtest.TicketsOwner, dbtest.PayDatabase},
	} {
		checkConnectDenied(t, pg.URL(c.role, c.database))
	}
}

// checkRoleAttributes asserts role can log in and has no elevated attributes.
func checkRoleAttributes(t *testing.T, super *pgx.Conn, role string) {
	t.Helper()
	var login, superuser, createDB, createRole bool
	if err := super.QueryRow(t.Context(),
		"SELECT rolcanlogin, rolsuper, rolcreatedb, rolcreaterole FROM pg_roles WHERE rolname = $1",
		role).Scan(&login, &superuser, &createDB, &createRole); err != nil {
		t.Fatalf("role %s: %v", role, err)
	}
	if !login || superuser || createDB || createRole {
		t.Errorf("role %s: login=%t superuser=%t createdb=%t createrole=%t, want true false false false",
			role, login, superuser, createDB, createRole)
	}
}

// checkConnectDenied asserts connecting to url fails with SQLSTATE 42501.
func checkConnectDenied(t *testing.T, url string) {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), url)
	if err == nil {
		_ = conn.Close(t.Context())
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		cfg, _ := pgx.ParseConfig(url)
		t.Errorf("%s connecting to %s: error = %v, want SQLSTATE 42501", cfg.User, cfg.Database, err)
	}
}

// Clause 4: an integration test reaches both databases and Redis.
func TestStack_ConnectsToBothDatabasesAndRedis(t *testing.T) {
	t.Parallel()
	pg := dbtest.StartPostgres(t)
	for _, url := range []string{pg.PayAppURL, pg.TicketsAppURL} {
		var one int
		if err := dbtest.Connect(t, url).QueryRow(t.Context(), "SELECT 1").Scan(&one); err != nil || one != 1 {
			t.Errorf("SELECT 1 = %d (%v), want 1", one, err)
		}
	}
	client := redisClient(t, dbtest.StartRedis(t))
	if err := client.Set(t.Context(), "p1-02:probe", "stack", 0).Err(); err != nil {
		t.Fatalf("SET: %v", err)
	}
	got, err := client.Get(t.Context(), "p1-02:probe").Result()
	if err != nil || got != "stack" {
		t.Errorf("GET = %q (%v), want stack", got, err)
	}
}

// redisClient connects to url and closes the client when the test ends.
func redisClient(t *testing.T, url string) *redis.Client {
	t.Helper()
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse Redis URL: %v", err)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// infoField returns the value of key in a Redis INFO reply.
func infoField(info, key string) string {
	for line := range strings.Lines(info) {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), key+":"); ok {
			return value
		}
	}
	return ""
}
