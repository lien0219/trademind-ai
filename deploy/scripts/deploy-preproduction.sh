#!/usr/bin/env bash
set -euo pipefail
umask 077

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE=(docker compose --project-name trademind-preproduction --env-file "$ROOT/.env" -f "$ROOT/deploy/preproduction/compose.yml")
MODE=startup
DRY_RUN=false
case "${1:-}" in
  --dry-run) DRY_RUN=true ;;
  --backend-only) MODE=migration ;;
  "") ;;
  *) echo "Usage: $0 [--dry-run|--backend-only]" >&2; exit 2 ;;
esac

node "$ROOT/scripts/preproduction-preflight.mjs" --mode "$MODE" >/dev/null
IMAGE_REFERENCES="$("${COMPOSE[@]}" config --format json | node "$ROOT/scripts/preproduction-preflight.mjs" --mode "$MODE" --resolve-compose)"
mapfile -t IMAGES <<< "$IMAGE_REFERENCES"
echo "[preproduction] isolated Compose configuration and immutable application references validated"
if $DRY_RUN; then
  exit 0
fi

command -v python3 >/dev/null || { echo "[preproduction] python3 missing" >&2; exit 1; }
"${COMPOSE[@]}" pull backend admin collector
bash "$ROOT/deploy/scripts/verify-application-images.sh" "${IMAGES[@]}"
if [[ "$MODE" == migration ]]; then
  "${COMPOSE[@]}" up -d --wait --wait-timeout 240 backend
  bash "$ROOT/deploy/scripts/check-preproduction-readiness.sh" --backend-only
else
  "${COMPOSE[@]}" up -d --wait --wait-timeout 240
  bash "$ROOT/deploy/scripts/check-preproduction-readiness.sh"
fi
