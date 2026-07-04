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
# default firefly:latest). Built once from the sibling checkout (Rust release
# build, several minutes on first boot); later starts reuse the cached image.
if ! docker image inspect firefly:latest >/dev/null 2>&1; then
	echo "Building the Firefly tracker image (first boot only, several minutes)…"
	docker build -t firefly:latest ../firefly
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
