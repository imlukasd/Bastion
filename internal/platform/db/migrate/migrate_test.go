package migrate_test

import (
	"crypto/sha256"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/imlukasd/Bastion/internal/platform/db/migrate"
)

// files builds an in-memory migrations directory from name → SQL.
func files(m map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, sql := range m {
		fsys[name] = &fstest.MapFile{Data: []byte(sql)}
	}
	return fsys
}

func TestLoad_SortsByVersionAndIgnoresOtherFiles(t *testing.T) {
	ms, err := migrate.Load(files(map[string]string{
		"0002_second.sql":    "SELECT 2;",
		"0001_first.sql":     "SELECT 1;",
		"README.md":          "not a migration",
		"nested/0003_no.sql": "SELECT 3;",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("Load returned %d migrations, want 2: %+v", len(ms), ms)
	}
	want := []struct {
		version  int
		name, sq string
	}{{1, "0001_first.sql", "SELECT 1;"}, {2, "0002_second.sql", "SELECT 2;"}}
	for i, w := range want {
		if ms[i].Version != w.version || ms[i].Name != w.name || ms[i].SQL != w.sq {
			t.Errorf("migration %d = {%d %q %q}, want {%d %q %q}", i, ms[i].Version, ms[i].Name, ms[i].SQL, w.version, w.name, w.sq)
		}
	}
}

func TestLoad_ChecksumIsSHA256OfFileBytes(t *testing.T) {
	const sql = "CREATE TABLE t (id int);\n"
	ms, err := migrate.Load(files(map[string]string{"0001_t.sql": sql}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := sha256.Sum256([]byte(sql)); ms[0].Checksum != want {
		t.Errorf("Checksum = %x, want %x", ms[0].Checksum, want)
	}
}

func TestLoad_RejectsBadNames(t *testing.T) {
	for _, name := range []string{"1_x.sql", "0001-x.sql", "0001_X.sql", "00001_x.sql", "0001_.sql", "0001.sql"} {
		t.Run(name, func(t *testing.T) {
			_, err := migrate.Load(files(map[string]string{name: "SELECT 1;"}))
			if !errors.Is(err, migrate.ErrBadName) {
				t.Errorf("Load(%s) error = %v, want ErrBadName", name, err)
			}
		})
	}
}

func TestLoad_RejectsDuplicateVersion(t *testing.T) {
	_, err := migrate.Load(files(map[string]string{"0001_a.sql": "SELECT 1;", "0001_b.sql": "SELECT 1;"}))
	if !errors.Is(err, migrate.ErrDuplicateVersion) {
		t.Errorf("Load error = %v, want ErrDuplicateVersion", err)
	}
}
