#!/usr/bin/env bash
# One-time codespace setup (postCreate): clone the Firefly sibling checkout —
# start.sh builds the firefly:latest image from it, which the orchestrator
# spawns per feed (WAYFINDER_FIREFLY_IMAGE, ADR 0012).
# Idempotent: a rebuild of the same codespace keeps the existing clone.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
sibling="$(dirname "$root")/firefly"

if [ -d "$sibling/.git" ]; then
	echo "Firefly checkout already present at $sibling — keeping it."
	exit 0
fi

echo "Cloning Firefly next to Wayfinder → $sibling"
# gh is authenticated with the codespace token; the devcontainer requests read
# access to the Firefly repo (customizations.codespaces.repositories).
gh repo clone ManuelRingwald/Firefly "$sibling" -- --depth 1
