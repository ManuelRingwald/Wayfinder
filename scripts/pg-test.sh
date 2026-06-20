#!/usr/bin/env bash
#
# Run the pkg/store tests against a throwaway PostgreSQL instance.
#
# The integration tests (store_integration_test.go) skip unless
# WAYFINDER_TEST_DB_URL points at a database. This script spins up a temporary
# Postgres cluster, runs the tests against it, and tears everything down — so the
# real schema + repositories can be exercised locally and in CI without Docker.
#
# Requirements: the PostgreSQL server binaries (initdb/pg_ctl, override via
# PGBIN) and, when run as root, a "postgres" system user (Postgres refuses to run
# as root). Extra args are passed to `go test`, e.g. `scripts/pg-test.sh -v`.
set -euo pipefail

PGBIN="${PGBIN:-/usr/lib/postgresql/16/bin}"
PORT="${PGPORT:-55432}"
WORKDIR="$(mktemp -d)"

# pg runs a command as the postgres user when invoked as root, else directly.
pg() {
	if [ "$(id -u)" = "0" ]; then su -s /bin/bash postgres -c "$1"; else bash -c "$1"; fi
}

cleanup() {
	pg "$PGBIN/pg_ctl -D '$WORKDIR/data' -m fast -w stop" >/dev/null 2>&1 || true
	rm -rf "$WORKDIR"
}
trap cleanup EXIT

[ "$(id -u)" = "0" ] && chown postgres:postgres "$WORKDIR"

pg "$PGBIN/initdb -D '$WORKDIR/data' -A trust -U postgres" >/dev/null
pg "$PGBIN/pg_ctl -D '$WORKDIR/data' -l '$WORKDIR/log' \
	-o '-p $PORT -k $WORKDIR -c listen_addresses=127.0.0.1' -w start" >/dev/null
pg "$PGBIN/createdb -h '$WORKDIR' -p $PORT wayfinder_test"

# Not `exec`: that would replace this shell and skip the cleanup trap, leaking the
# temporary server. Run go test normally; its exit code propagates (set -e).
export WAYFINDER_TEST_DB_URL="postgres://postgres@127.0.0.1:$PORT/wayfinder_test?sslmode=disable"
go test ./pkg/store/... "$@"
