// Package store provides PostgreSQL-backed persistence for Wayfinder 2.0's
// multi-tenant configuration and identity (ADR 0005/0006): tenants, users,
// feeds, subscriptions, view configs and entitlements.
//
// Why a database at all: in 1.x Wayfinder read its (single-tenant) config once
// from the environment at startup. 2.0 must serve many isolated tenants whose
// configuration changes at runtime, so that state moves into PostgreSQL while
// the application stays stateless (ADR 0006 §6). Only infrastructure secrets —
// such as the connection string — remain in the environment (12-Factor).
//
// The schema lives in embedded SQL migrations (migrations/*.sql) applied by a
// small in-house, transactional runner (see migrate.go), so a single binary can
// bring its database up to date at startup with no external CLI and with
// version-controlled baselines (config management, CLAUDE.md §7), and without
// dragging in heavy migration-library dependencies.
package store

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNoDSN is returned by DSNFromEnv when WAYFINDER_DB_URL is unset.
var ErrNoDSN = errors.New("store: WAYFINDER_DB_URL is not set")

// DSNFromEnv reads the PostgreSQL connection string from WAYFINDER_DB_URL. The
// DSN is an infrastructure secret and therefore comes from the environment, not
// the tenant database (ADR 0006 §6, 12-Factor).
func DSNFromEnv() (string, error) {
	dsn := os.Getenv("WAYFINDER_DB_URL")
	if dsn == "" {
		return "", ErrNoDSN
	}
	return dsn, nil
}

// Open creates a pgx connection pool for runtime queries. The caller owns the
// pool and must Close it. Open pings the database so a misconfigured connection
// fails fast at startup rather than on the first query.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return pool, nil
}
