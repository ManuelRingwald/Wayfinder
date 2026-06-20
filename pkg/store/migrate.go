package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsDir is the path of the embedded migrations within migrationsFS.
const migrationsDir = "migrations"

// downMarker separates the forward ("up") SQL of a migration file from its
// optional rollback ("down") SQL. The runner is forward-only: it applies the up
// section; the down section is kept for humans/operators (manual rollback).
const downMarker = "-- +migrate down"

// migration is one versioned SQL migration parsed from the embedded files.
type migration struct {
	version int64  // numeric prefix of the file name (e.g. 00001_init.sql -> 1)
	name    string // remainder of the file name without extension (e.g. "init")
	up      string // forward SQL (everything above the down marker)
}

// Migrate applies every embedded migration not yet recorded in
// schema_migrations, in ascending version order, each inside its own
// transaction (PostgreSQL has transactional DDL, so a failed migration leaves no
// partial schema). It is idempotent and safe to call on every startup.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}

	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    BIGINT PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("store: ensure schema_migrations: %w", err)
	}

	for _, m := range migs {
		applied, err := migrationApplied(ctx, pool, m.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, pool, m); err != nil {
			return fmt.Errorf("store: apply migration %d (%s): %w", m.version, m.name, err)
		}
	}
	return nil
}

// migrationApplied reports whether the given version is already recorded.
func migrationApplied(ctx context.Context, pool *pgxpool.Pool, version int64) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("store: check migration %d: %w", version, err)
	}
	return exists, nil
}

// applyMigration runs one migration's up SQL and records it, atomically.
func applyMigration(ctx context.Context, pool *pgxpool.Pool, m migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful commit

	if _, err := tx.Exec(ctx, m.up); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.version, m.name,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// loadMigrations reads and parses the embedded migrations, sorted by version. It
// rejects malformed names and duplicate versions so a packaging mistake fails
// loudly rather than silently skipping a migration.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("store: read migrations dir: %w", err)
	}

	var migs []migration
	seen := make(map[int64]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		version, name, err := parseMigrationName(e.Name())
		if err != nil {
			return nil, err
		}
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("store: duplicate migration version %d (%s and %s)", version, prev, e.Name())
		}
		seen[version] = e.Name()

		raw, err := fs.ReadFile(migrationsFS, path.Join(migrationsDir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("store: read migration %s: %w", e.Name(), err)
		}
		up := strings.TrimSpace(upSection(string(raw)))
		if up == "" {
			return nil, fmt.Errorf("store: migration %s has an empty up section", e.Name())
		}
		migs = append(migs, migration{version: version, name: name, up: up})
	}

	sort.Slice(migs, func(i, j int) bool { return migs[i].version < migs[j].version })
	return migs, nil
}

// parseMigrationName extracts the version and name from "NNN_some_name.sql".
func parseMigrationName(filename string) (int64, string, error) {
	base := strings.TrimSuffix(filename, ".sql")
	i := strings.IndexByte(base, '_')
	if i <= 0 || i == len(base)-1 {
		return 0, "", fmt.Errorf("store: migration name must be <version>_<name>.sql, got %q", filename)
	}
	version, err := strconv.ParseInt(base[:i], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("store: migration %q has a non-numeric version: %w", filename, err)
	}
	if version <= 0 {
		return 0, "", fmt.Errorf("store: migration %q version must be positive", filename)
	}
	return version, base[i+1:], nil
}

// upSection returns the forward SQL of a migration: everything before the down
// marker, or the whole text if there is no down section.
func upSection(s string) string {
	if i := strings.Index(s, downMarker); i >= 0 {
		return s[:i]
	}
	return s
}
