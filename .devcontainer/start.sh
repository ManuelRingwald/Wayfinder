#!/usr/bin/env bash
# Bring the browser-only ORCHESTRATED stack up (postStart, runs on every
# codespace start): Postgres + Wayfinder + the orchestrator control plane
# (ADR 0012) — the orchestrator auto-spawns one Firefly tracker per subscribed
# feed, exactly like the production-shaped E2E harness. Host networking works
# here because a codespace IS a Linux host (docker-in-docker): every host-net
# container shares one network namespace, where CAT062/UDP-multicast is
# delivered locally. (The "needs Linux" caveat in DOCKER.md is about Docker
# Desktop's VM boundary — a codespace has none.)
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

if [ ! -d "../firefly/.git" ]; then
	echo "Firefly checkout missing — running setup first."
	bash .devcontainer/setup.sh
fi

# The per-feed Firefly image the orchestrator spawns (WAYFINDER_FIREFLY_IMAGE,
# default firefly:latest), built from the sibling checkout. We pull the checkout
# and (re)build the image on EVERY start — deliberately NOT "build once, then keep
# the cache". A once-built image silently goes stale against Wayfinder's main: a
# newer FIREFLY_SOURCES variant (e.g. adsb_aggregator, contract v1.5.0) is then
# rejected by the old tracker, which crash-loops with "unknown variant" and the
# feed never turns green — no track ever reaches the map. Thanks to Docker's layer
# cache this is a seconds-long no-op when Firefly is unchanged; the real (several-
# minute Rust) build only runs when Firefly's main actually moved.
echo "Syncing the Firefly checkout…"
git -C ../firefly pull --ff-only || echo "  (pull skipped — building from the current checkout)"
echo "Building the Firefly tracker image (cached & fast unless Firefly changed)…"
# Keep a rebuild failure (e.g. Firefly's main momentarily red) from bricking the
# whole start: bring the stack — and with it the browser UI — up anyway, falling
# back to the previously built image, and say so loudly. Only a build failure with
# NO image at all leaves trackers unspawnable (the UI still comes up).
if ! docker build -t firefly:latest ../firefly; then
	if docker image inspect firefly:latest >/dev/null 2>&1; then
		echo "  ⚠ Firefly rebuild failed — continuing with the previously built firefly:latest."
	else
		echo "  ⚠ Firefly build failed and no firefly:latest exists — feeds cannot spawn a tracker until this is fixed."
	fi
fi

# Codespace-local secrets (compose reads .env for substitution; gitignored,
# survive restarts of THIS codespace): a stable session key keeps logins across
# restarts, the secret key enables credentialled sources (OpenSky client
# credentials via the admin UI's feed-secrets endpoints — without it the
# secrets API answers 503).
#
# Encoding matters, and the two keys differ:
#   - SESSION_KEY is consumed as raw bytes (HMAC accepts any length), so hex is fine.
#   - SECRET_KEY must be base64-encoded 32 bytes: the server parses it with
#     secret.KeyFromBase64 (AES-256). `openssl rand -hex 32` yielded a 64-char
#     hex string that base64-decodes to 48 bytes ≠ 32 → rejected as invalid, and
#     the secret store silently stayed disabled (issue #171). `-base64 32` emits
#     exactly 32 bytes base64-encoded, which the server accepts.
if [ ! -f .env ]; then
	{
		echo "WAYFINDER_SESSION_KEY=$(openssl rand -hex 32)"
		echo "WAYFINDER_SECRET_KEY=$(openssl rand -base64 32)"
	} >.env
	echo "Generated codespace-local .env (session-signing + secret key)."
fi

echo "Starting the orchestrated stack (Postgres + Wayfinder + Orchestrator)…"
docker compose -f docker-compose.orchestrated.yml up --build -d

# A freshly (re)built firefly:latest does NOT by itself replace already-running
# spawned trackers: the orchestrator keys its drift check on the image *name*
# (firefly:latest), not the digest, so an old-image container reads as "converged"
# and is left crash-looping. Remove the spawned per-feed trackers once so the
# orchestrator respawns them from the current image on its next reconcile
# (≤ WAYFINDER_ORCHESTRATOR_INTERVAL). A brief empty-map gap on start is fine;
# no-op when none exist yet (fresh codespace).
echo "Refreshing spawned Firefly trackers so they pick up the current image…"
docker ps -aq --filter "label=wayfinder.managed=true" | xargs -r docker rm -f || true

echo -n "Waiting for Wayfinder /health "
for _ in $(seq 1 60); do
	if curl -fsS http://localhost:8080/health >/dev/null 2>&1; then
		echo " — up."
		break
	fi
	echo -n "."
	sleep 2
done

url="http://localhost:8081"
if [ -n "${CODESPACE_NAME:-}" ]; then
	url="https://${CODESPACE_NAME}-8081.${GITHUB_CODESPACES_PORT_FORWARDING_DOMAIN:-app.github.dev}"
fi

cat <<EOF

──────────────────────────────────────────────────────────────────────
 Wayfinder läuft (orchestrierter Stack — Auto-Spawn je Feed aktiv).

   ${url}/admin   (Erst-Login: admin / admin → Pflicht-Passwortwechsel)

 Einrichtung wie im echten Betrieb (Details: docs/CODESPACES.md):
   1. Mandant anlegen.
   2. Feed anlegen (Endpoint einfach auto-allokieren lassen).
   3. Feed-Quellen setzen (z. B. adsb_opensky + BBox) und die
      OpenSky-Zugangsdaten über den Quellen-Dialog hinterlegen.
   4. Mandant auf den Feed abonnieren, Ansicht speichern.
   → Der Orchestrator spawnt automatisch einen Firefly je Feed;
     Tracks erscheinen auf ${url}/
──────────────────────────────────────────────────────────────────────
EOF
