#!/usr/bin/env bash
#
# ORCH-5c end-to-end acceptance harness (ADR 0012).
#
# Drives the full auto-orchestration chain on a real Docker host and asserts each
# checkpoint: assign a feed → the orchestrator spawns a Firefly tracker for it →
# CAT062/UDP-multicast → the ASD receives tracks → unsubscribe tears it down.
#
# It seeds the catalogue directly in Postgres (a tenant + feed + subscription) so
# the run targets the orchestrator path without the admin-auth choreography; the
# admin API is exercised by its own tests.
#
# Two modes:
#   --mode empty        (default) feed WITHOUT live sources → the spawned Firefly
#                       idles with an honest EMPTY SKY (Firefly ADR 0030) but
#                       emits the CAT065 heartbeat. Fully offline; proves
#                       spawn → multicast → ASD liveness → cleanup
#                       (checkpoints 1, 2, 5*, 8; 5 asserts heartbeats, not tracks).
#   --mode opensky-anon feed WITH an anonymous adsb_opensky source → exercises the
#                       FIREFLY_SOURCES live path. Needs outbound network to
#                       OpenSky; the authenticated variant (credential →
#                       FIREFLY_SOURCE_0_SECRET) is a manual step in
#                       docs/E2E-ABNAHME.md.
#
# Requirements: a running Docker daemon, docker compose v2, and the Firefly image
# available locally (build it from the Firefly repo: `docker build -t firefly:latest .`)
# or set WAYFINDER_FIREFLY_IMAGE.
#
# Usage:
#   scripts/e2e-orchestrated.sh [--mode empty|opensky-anon] [--keep]
#
#   --keep   leave the stack and DB running after the run (for inspection);
#            default tears everything down.

set -euo pipefail

# --- configuration ----------------------------------------------------------
MODE="empty"
KEEP=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="${2:-}"; shift 2 ;;
    --keep) KEEP=1; shift ;;
    -h|--help) sed -n '2,40p' "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
if [[ "$MODE" != "empty" && "$MODE" != "opensky-anon" ]]; then
  echo "invalid --mode: $MODE (want: empty | opensky-anon)" >&2; exit 2
fi

cd "$(dirname "$0")/.."

COMPOSE=(docker compose -f docker-compose.orchestrated.yml)
FIREFLY_IMAGE="${WAYFINDER_FIREFLY_IMAGE:-firefly:latest}"
GROUP="${FIREFLY_CAT062_GROUP:-239.255.0.62}"
PORT="${FIREFLY_CAT062_PORT:-8600}"
HEALTH_URL="http://127.0.0.1:8080"
SPAWN_TIMEOUT=90   # seconds to wait for the orchestrator to spawn a container
ASD_TIMEOUT=45     # seconds to wait for the first tracks to reach the ASD
CLEANUP_TIMEOUT=45 # seconds to wait for orphan teardown

fail() { echo "  ✗ FAIL: $*" >&2; exit 1; }
pass() { echo "  ✓ $*"; }
info() { echo "→ $*"; }

psql_q() { "${COMPOSE[@]}" exec -T db psql -U wayfinder -d wayfinder -tAc "$1"; }

# --- cleanup ----------------------------------------------------------------
cleanup() {
  local code=$?
  if [[ "$KEEP" -eq 1 ]]; then
    info "leaving the stack up (--keep). Tear down with: ${COMPOSE[*]} down -v"
  else
    info "tearing down the stack"
    "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
    # Belt-and-braces: remove any per-feed tracker the orchestrator left behind.
    docker ps -aq --filter "label=wayfinder.managed=true" 2>/dev/null | xargs -r docker rm -f >/dev/null 2>&1 || true
  fi
  exit "$code"
}
trap cleanup EXIT

# --- preflight --------------------------------------------------------------
info "preflight"
docker info >/dev/null 2>&1 || fail "Docker daemon not reachable"
docker image inspect "$FIREFLY_IMAGE" >/dev/null 2>&1 \
  || fail "Firefly image '$FIREFLY_IMAGE' not found — build it from the Firefly repo (docker build -t firefly:latest .) or set WAYFINDER_FIREFLY_IMAGE"
pass "Docker daemon up, Firefly image '$FIREFLY_IMAGE' present"

# --- bring up the stack -----------------------------------------------------
info "building and starting db + server + orchestrator"
WAYFINDER_FIREFLY_IMAGE="$FIREFLY_IMAGE" "${COMPOSE[@]}" up -d --build

info "waiting for the server schema (migrations) to be ready"
for _ in $(seq 1 60); do
  if psql_q "SELECT 1 FROM information_schema.tables WHERE table_name='feeds'" 2>/dev/null | grep -q 1; then
    break
  fi
  sleep 2
done
psql_q "SELECT 1 FROM information_schema.tables WHERE table_name='feeds'" | grep -q 1 \
  || fail "the feeds table never appeared — did the wayfinder server start and migrate?"
pass "schema ready"

# --- seed the catalogue -----------------------------------------------------
if [[ "$MODE" == "empty" ]]; then
  SRC='[]'
else
  SRC='[{"type":"adsb_opensky","bbox":{"min_lat":47,"min_lon":5,"max_lat":55,"max_lon":16}}]'
fi

info "seeding tenant + feed ($MODE) + subscription"
psql_q "INSERT INTO tenants (slug, name) VALUES ('e2e', 'E2E Test Tenant')
        ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name" >/dev/null
TENANT_ID="$(psql_q "SELECT id FROM tenants WHERE slug = 'e2e'")"
FEED_ID="$(psql_q "INSERT INTO feeds (name, multicast_group, port, source_config)
        VALUES ('e2e-feed', '$GROUP', $PORT, '$SRC'::jsonb)
        ON CONFLICT (name) DO UPDATE
          SET source_config = EXCLUDED.source_config,
              multicast_group = EXCLUDED.multicast_group,
              port = EXCLUDED.port
        RETURNING id")"
psql_q "INSERT INTO subscriptions (tenant_id, feed_id) VALUES ($TENANT_ID, $FEED_ID)
        ON CONFLICT DO NOTHING" >/dev/null
[[ -n "$FEED_ID" ]] || fail "could not seed the feed"
pass "seeded feed id=$FEED_ID (tenant id=$TENANT_ID)"

CONTAINER="wayfinder-firefly-feed-$FEED_ID"

# --- checkpoint 1: the orchestrator spawns a container ----------------------
info "checkpoint 1 — orchestrator spawns the tracker container"
spawned=""
for _ in $(seq 1 $((SPAWN_TIMEOUT / 3))); do
  spawned="$(docker ps --filter "label=wayfinder.feed_id=$FEED_ID" --format '{{.Names}}' 2>/dev/null || true)"
  [[ -n "$spawned" ]] && break
  sleep 3
done
[[ -n "$spawned" ]] || fail "no container with label wayfinder.feed_id=$FEED_ID after ${SPAWN_TIMEOUT}s"
pass "container running: $spawned"

# --- checkpoint 2: the container carries the right config -------------------
info "checkpoint 2 — container env matches the spec"
ENVOUT="$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$CONTAINER")"
echo "$ENVOUT" | grep -q "^FIREFLY_CAT062_GROUP=$GROUP$" || fail "FIREFLY_CAT062_GROUP not set to $GROUP"
echo "$ENVOUT" | grep -q "^FIREFLY_CAT062_PORT=$PORT$" || fail "FIREFLY_CAT062_PORT not set to $PORT"
if [[ "$MODE" == "empty" ]]; then
  # No sources → the EXPLICIT empty contract; never a placeholder scene
  # (Firefly ADR 0030).
  echo "$ENVOUT" | grep -q "^FIREFLY_SOURCES=\[\]$" || fail "source-less feed should carry FIREFLY_SOURCES=[]"
  pass "endpoint + FIREFLY_SOURCES=[] present (empty sky contract)"
else
  echo "$ENVOUT" | grep -q "^FIREFLY_SOURCES=" || fail "opensky feed should carry FIREFLY_SOURCES"
  # An anonymous source carries no cred_env and emits no secret value.
  if echo "$ENVOUT" | grep -q "^FIREFLY_SOURCE_0_SECRET="; then
    fail "anonymous source must not emit a credential env"
  fi
  pass "endpoint + FIREFLY_SOURCES present, no credential env"
fi

# --- checkpoint 5: the feed reaches the ASD ---------------------------------
# empty mode: a source-less Firefly emits NO tracks by design (empty sky) —
# liveness is proven by the CAT065 heartbeat crossing into the ASD.
# opensky-anon: real tracks are expected (best-effort; traffic-dependent).
if [[ "$MODE" == "empty" ]]; then
  info "checkpoint 5 — heartbeat reaches the ASD (server /metrics)"
  METRIC_NAME='wayfinder_cat065_heartbeats_received_total'
else
  info "checkpoint 5 — tracks reach the ASD (server /metrics)"
  METRIC_NAME='wayfinder_cat062_tracks_received_total'
fi
got_signal=0
for _ in $(seq 1 $((ASD_TIMEOUT / 3))); do
  metric="$(curl -fsS "$HEALTH_URL/metrics" 2>/dev/null | grep -E "^${METRIC_NAME} " | awk '{print $2}' || true)"
  count="${metric%.*}"  # drop any fractional part so the integer test is safe
  if [[ "$count" =~ ^[0-9]+$ && "$count" -gt 0 ]]; then got_signal=1; break; fi
  sleep 3
done
if [[ "$got_signal" -eq 1 ]]; then
  pass "ASD received the feed signal (${METRIC_NAME} > 0)"
elif [[ "$MODE" == "empty" ]]; then
  fail "no CAT065 heartbeat within ${ASD_TIMEOUT}s — multicast not crossing the host? check 'docker logs $CONTAINER'"
else
  echo "  ⚠ WARN: no CAT062 tracks observed within ${ASD_TIMEOUT}s." >&2
  echo "    For --mode opensky-anon it may just be sparse traffic in the bbox." >&2
fi

# --- checkpoint 8: unsubscribe tears the tracker down -----------------------
info "checkpoint 8 — unsubscribe triggers orphan cleanup"
psql_q "DELETE FROM subscriptions WHERE feed_id = $FEED_ID" >/dev/null
gone=0
for _ in $(seq 1 $((CLEANUP_TIMEOUT / 3))); do
  if [[ -z "$(docker ps --filter "label=wayfinder.feed_id=$FEED_ID" --format '{{.Names}}' 2>/dev/null || true)" ]]; then
    gone=1; break
  fi
  sleep 3
done
[[ "$gone" -eq 1 ]] || fail "container for feed $FEED_ID still running ${CLEANUP_TIMEOUT}s after unsubscribe"
pass "tracker torn down after unsubscribe"

echo
echo "✅ E2E acceptance ($MODE) passed."
