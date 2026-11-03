package migrate_test

import (
	"crypto/sha256"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/imlukasd/Bastion/internal/platform/db/dbtest"
	"github.com/imlukasd/Bastion/internal/platform/db/migrate"
)

// scratch starts a server and returns a connection, as pay_owner, to a fresh
// database it owns (the role migrations run as, TD §8.3).
func scratch(t *testing.T) *pgx.Conn {
	t.Helper()
	t.Parallel()
	pg := dbtest.StartPostgresUnmigrated(t)
	super := dbtest.Connect(t, pg.SuperuserURL)
	if _, err := super.Exec(t.Context(), "CREATE DATABASE scratch OWNER "+dbtest.PayOwner); err != nil {
		t.Fatalf("create database: %v", err)
	}
	return dbtest.Connect(t, pg.URL(dbtest.PayOwner, "scratch"))
}

// load parses an in-memory migrations directory.
func load(t *testing.T, m map[string]string) []migrate.Migration {
	t.Helper()
	ms, err := migrate.Load(files(m))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return ms
}

// up runs Up and fails the test on an error.
func up(t *testing.T, conn *pgx.Conn, ms []migrate.Migration) []int {
	t.Helper()
	applied, err := migrate.Up(t.Context(), conn, ms)
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	return applied
}

// recorded returns the versions in schema_migrations.
func recorded(t *testing.T, conn *pgx.Conn) []int {
	t.Helper()
	rows, err := conn.Query(t.Context(), "SELECT version FROM public.schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	versions, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	out := make([]int, len(versions))
	for i, v := range versions {
		out[i] = int(v)
	}
	return out
}

// tableExists reports whether public.<name> exists.
func tableExists(t *testing.T, conn *pgx.Conn, name string) bool {
	t.Helper()
	var exists bool
	if err := conn.QueryRow(t.Context(), "SELECT to_regclass('public.' || $1) IS NOT NULL", name).Scan(&exists); err != nil {
		t.Fatalf("to_regclass: %v", err)
	}
	return exists
}

// count returns the number of rows in public.<table>.
func count(t *testing.T, conn *pgx.Conn, table string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(t.Context(), "SELECT count(*) FROM public."+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// The two first migrations; 0002 needs 0001's table, so it proves the order.
const (
	sqlA = "CREATE TABLE a (id int); INSERT INTO a VALUES (1);"
	sqlB = "INSERT INTO a VALUES (2); CREATE TABLE b (id int);"
)

// with returns 0001 and 0002 plus extra files (an extra file replaces one of the same name).
func with(extra map[string]string) map[string]string {
	m := map[string]string{"0001_a.sql": sqlA, "0002_b.sql": sqlB}
	maps.Copy(m, extra)
	return m
}

func TestUp_AppliesInOrderOnce(t *testing.T) {
	conn := scratch(t)
	ms := load(t, with(nil))

	if got := up(t, conn, ms); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("first Up applied %v, want [1 2]", got)
	}
	if got := count(t, conn, "a"); got != 2 {
		t.Errorf("rows in a = %d, want 2", got)
	}
	if !tableExists(t, conn, "b") {
		t.Error("table b missing after 0002")
	}
	for _, m := range ms {
		var name string
		var checksum []byte
		if err := conn.QueryRow(t.Context(),
			"SELECT name, checksum FROM public.schema_migrations WHERE version = $1", m.Version).Scan(&name, &checksum); err != nil {
			t.Fatalf("schema_migrations row %d: %v", m.Version, err)
		}
		if want := sha256.Sum256([]byte(m.SQL)); name != m.Name || !slices.Equal(checksum, want[:]) {
			t.Errorf("row %d = (%s, %x), want (%s, %x)", m.Version, name, checksum, m.Name, want)
		}
	}

	if got := up(t, conn, ms); len(got) != 0 {
		t.Errorf("second Up applied %v, want nothing", got)
	}
	if got := count(t, conn, "a"); got != 2 {
		t.Errorf("rows in a after second Up = %d, want 2", got)
	}
	if got := recorded(t, conn); !slices.Equal(got, []int{1, 2}) {
		t.Errorf("schema_migrations = %v, want [1 2]", got)
	}
}

func TestUp_RefusesEditedMigration(t *testing.T) {
	conn := scratch(t)
	up(t, conn, load(t, map[string]string{"0001_a.sql": sqlA}))

	edited := with(nil)
	edited["0001_a.sql"] = "CREATE TABLE a (id int); INSERT INTO a VALUES (100);"
	_, err := migrate.Up(t.Context(), conn, load(t, edited))
	if !errors.Is(err, migrate.ErrEdited) || !strings.Contains(err.Error(), "0001") {
		t.Fatalf("Up error = %v, want ErrEdited for 0001", err)
	}
	if got := recorded(t, conn); !slices.Equal(got, []int{1}) {
		t.Errorf("schema_migrations = %v, want [1]", got)
	}
	if tableExists(t, conn, "b") {
		t.Error("0002 ran although 0001 was edited")
	}
}

func TestUp_RefusesUnknownApplied(t *testing.T) {
	conn := scratch(t)
	up(t, conn, load(t, with(nil)))

	_, err := migrate.Up(t.Context(), conn, load(t, map[string]string{
		"0001_a.sql": sqlA,
		"0003_c.sql": "CREATE TABLE c (id int);",
	}))
	if !errors.Is(err, migrate.ErrUnknownApplied) || !strings.Contains(err.Error(), "0002") {
		t.Fatalf("Up error = %v, want ErrUnknownApplied for 0002", err)
	}
	if tableExists(t, conn, "c") {
		t.Error("0003 ran although 0002 has no file")
	}
}

func TestUp_RefusesOutOfOrder(t *testing.T) {
	conn := scratch(t)
	up(t, conn, load(t, map[string]string{
		"0001_a.sql": sqlA,
		"0003_c.sql": "CREATE TABLE c (id int);",
	}))

	_, err := migrate.Up(t.Context(), conn, load(t, with(map[string]string{"0003_c.sql": "CREATE TABLE c (id int);"})))
	if !errors.Is(err, migrate.ErrOutOfOrder) || !strings.Contains(err.Error(), "0002") {
		t.Fatalf("Up error = %v, want ErrOutOfOrder for 0002", err)
	}
	if got := recorded(t, conn); !slices.Equal(got, []int{1, 3}) {
		t.Errorf("schema_migrations = %v, want [1 3]", got)
	}
	if tableExists(t, conn, "b") {
		t.Error("0002 ran out of order")
	}
}

func TestUp_FailedMigrationRollsBack(t *testing.T) {
	conn := scratch(t)
	ms := load(t, map[string]string{
		"0001_a.sql": sqlA,
		"0002_b.sql": "CREATE TABLE b (id int); SELECT * FROM missing_table;",
	})

	applied, err := migrate.Up(t.Context(), conn, ms)
	if err == nil || !strings.Contains(err.Error(), "0002") {
		t.Fatalf("Up error = %v, want a failure naming 0002", err)
	}
	if !slices.Equal(applied, []int{1}) {
		t.Errorf("Up applied %v, want [1]", applied)
	}
	if got := recorded(t, conn); !slices.Equal(got, []int{1}) {
		t.Errorf("schema_migrations = %v, want [1]", got)
	}
	if tableExists(t, conn, "b") {
		t.Error("table b exists: the failed migration was not rolled back")
	}
}
