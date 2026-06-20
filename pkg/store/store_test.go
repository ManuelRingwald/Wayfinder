package store

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

// schemaTables are the tables the initial migration must create and drop.
var schemaTables = []string{"tenants", "users", "feeds", "subscriptions", "view_configs", "entitlements"}

func TestDSNFromEnv(t *testing.T) {
	t.Setenv("WAYFINDER_DB_URL", "")
	if _, err := DSNFromEnv(); !errors.Is(err, ErrNoDSN) {
		t.Fatalf("expected ErrNoDSN for empty env, got %v", err)
	}

	const want = "postgres://u:p@localhost:5432/wayfinder"
	t.Setenv("WAYFINDER_DB_URL", want)
	got, err := DSNFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("DSNFromEnv = %q, want %q", got, want)
	}
}

// TestLoadMigrations checks the embedded migration set parses cleanly and the
// up section creates every schema table while excluding the down section. This
// is deliberately database-free: schema *application* is exercised by the
// integration tests (WF2-10.3, CI with a live Postgres), while this guards
// against drift and packaging mistakes in any sandbox.
func TestLoadMigrations(t *testing.T) {
	migs, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if len(migs) != 2 {
		t.Fatalf("loaded %d migrations, want 2", len(migs))
	}

	// Migrations are returned in ascending version order.
	if migs[0].version != 1 || migs[1].version != 2 {
		t.Fatalf("versions = [%d %d], want [1 2]", migs[0].version, migs[1].version)
	}

	init := migs[0]
	if init.name != "init" {
		t.Fatalf("first migration name = %q, want init", init.name)
	}
	for _, tbl := range schemaTables {
		if !strings.Contains(init.up, "CREATE TABLE "+tbl+" ") {
			t.Errorf("init up section does not CREATE TABLE %s", tbl)
		}
	}
	if strings.Contains(init.up, "DROP TABLE") {
		t.Error("init up section unexpectedly contains DROP TABLE (down marker not honoured)")
	}
}

// TestInitMigrationDownDropsEveryTable verifies up/down symmetry directly from
// the embedded file.
func TestInitMigrationDownDropsEveryTable(t *testing.T) {
	raw, err := fs.ReadFile(migrationsFS, "migrations/00001_init.sql")
	if err != nil {
		t.Fatalf("read embedded migration: %v", err)
	}
	idx := strings.Index(string(raw), downMarker)
	if idx < 0 {
		t.Fatal("migration is missing the down marker")
	}
	down := string(raw)[idx:]
	for _, tbl := range schemaTables {
		if !strings.Contains(down, "DROP TABLE IF EXISTS "+tbl+";") {
			t.Errorf("down section does not DROP TABLE %s", tbl)
		}
	}
}

func TestParseMigrationName(t *testing.T) {
	okCases := map[string]struct {
		version int64
		name    string
	}{
		"00001_init.sql":       {1, "init"},
		"42_add_audit_log.sql": {42, "add_audit_log"},
		"007_feeds.sql":        {7, "feeds"},
	}
	for filename, want := range okCases {
		v, n, err := parseMigrationName(filename)
		if err != nil {
			t.Errorf("%s: unexpected error %v", filename, err)
			continue
		}
		if v != want.version || n != want.name {
			t.Errorf("%s -> {%d %q}, want {%d %q}", filename, v, n, want.version, want.name)
		}
	}

	badCases := []string{"init.sql", "_init.sql", "abc_init.sql", "00001_.sql", "0_init.sql", "-1_init.sql"}
	for _, filename := range badCases {
		if _, _, err := parseMigrationName(filename); err == nil {
			t.Errorf("%s: expected an error, got nil", filename)
		}
	}
}

func TestUpSection(t *testing.T) {
	withDown := "CREATE TABLE a ();\n" + downMarker + "\nDROP TABLE a;"
	if got := upSection(withDown); strings.Contains(got, "DROP TABLE") || !strings.Contains(got, "CREATE TABLE a") {
		t.Errorf("upSection did not split on the down marker: %q", got)
	}
	noDown := "CREATE TABLE a ();"
	if got := upSection(noDown); got != noDown {
		t.Errorf("upSection without marker = %q, want unchanged", got)
	}
}
